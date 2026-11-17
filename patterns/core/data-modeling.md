# Data Modeling

> How to shape data for a domain, as distinct from how to write the code that
> touches it. Conventions for that code: [database.md](database.md).

Most decisions in this blueprint are reversible in an afternoon. Data shape is
not. A schema that conflates what happened with what you concluded, or that
overwrites rather than supersedes, cannot be fixed later by better code: the
information you would need was never written down.

Reach for these shapes when a new table calls for them. They are not a
requirement on tables you already have, and most tables need none of this.
Examples use `OrgScoped`; with `tenancy: personal`, carry `user_id` instead
([database.md](database.md#multi-tenancy)). Every snippet was run on SQLite;
anything PostgreSQL-only is marked.

## Table of Contents

- [Evidence and Derivation](#evidence-and-derivation)
- [Temporal Data](#temporal-data)
- [Superseding, Not Overwriting](#superseding-not-overwriting)
- [Entities, Edges, Evidence](#entities-edges-evidence)
- [Identity Resolution](#identity-resolution)
- [Closed Vocabularies First](#closed-vocabularies-first)
- [Querying a Graph in Postgres](#querying-a-graph-in-postgres)
- [Deletion](#deletion)
- [When You Need a Graph Database](#when-you-need-a-graph-database)

---

## Evidence and Derivation

When a domain interprets its input, split it into two kinds of table and never
mix them.

**Evidence** is what actually happened, as recorded at the time. A message
someone sent, a file they uploaded, a click, a payment. It is append-only and
immutable. If it turns out to be wrong, it is still what happened.

**Derivation** is what you concluded from evidence. A summary, an extracted
request, a computed score, a category. It is revisable, rebuildable, and always
traceable to the evidence that produced it.

```
message          ← evidence: immutable, user-authored
message_edit     ← evidence: the edit is itself an event, not a mutation
fact             ← derivation: revisable, cites evidence
fact_evidence    ← the citation
```

The test: **can you delete every derivation table and rebuild it from
evidence?** If not, something irreplaceable is stored in a table you treat as
disposable, and a backfill will destroy it.

This is why an edit is a new row rather than an `UPDATE`. The moment you
overwrite the body of a message, you lose the ability to explain why a
conclusion drawn last week no longer follows from the text in front of you.

Neither engine stops an update, so immutability lives in the model: an evidence
model has a `create()` and no `update()`.

---

## Temporal Data

Two different times, routinely confused, and the confusion is unrecoverable
later:

- **Valid time**: when the fact was true in the world. The decision was made on
  Tuesday.
- **Transaction time**: when the system learned it. You recorded it on Friday.

You need both whenever the system can learn things late or be corrected, which
is any system that interprets rather than merely records.

```python
# app/models/fact.py
LIVE = sa.text('valid_to IS NULL AND retracted_at IS NULL')


class Fact(OrgScoped, BaseModel):
    __tablename__ = 'fact'

    kind = db.Column(db.String(50), nullable=False)
    body = db.Column(JSONColumn, nullable=False, default=dict)
    source = db.Column(db.String(20), nullable=False)    # user | model | rule | import
    valid_from = db.Column(db.DateTime(timezone=True), nullable=False, default=utcnow)
    valid_to = db.Column(db.DateTime(timezone=True), nullable=True)      # NULL = still true
    recorded_at = db.Column(db.DateTime(timezone=True), nullable=False, default=utcnow)
    retracted_at = db.Column(db.DateTime(timezone=True), nullable=True)  # stopped believing it
    superseded_by = db.Column(BigIntFK, db.ForeignKey('fact.id'), nullable=True)

    # Current beliefs about current reality. A partial index needs both arguments.
    __table_args__ = (db.Index('ix_fact_current', 'org_id', 'kind',
                               sqlite_where=LIVE, postgresql_where=LIVE),)

    @classmethod
    def current(cls, kind: str):
        """What is true now, as far as we currently believe?"""
        return db.session.scalars(sa.select(cls).where(
            cls.kind == kind, cls.valid_to.is_(None), cls.retracted_at.is_(None))).all()

    @classmethod
    def as_of(cls, kind: str, valid_at: datetime, known_at: datetime):
        """What did we believe at known_at about the world at valid_at?"""
        return db.session.scalars(sa.select(cls).where(
            cls.kind == kind,
            cls.valid_from <= valid_at,
            sa.or_(cls.valid_to.is_(None), cls.valid_to > valid_at),
            cls.recorded_at <= known_at,
            sa.or_(cls.retracted_at.is_(None), cls.retracted_at > known_at))).all()
```

`as_of` is the query that justifies the whole design. "What did we think the
pricing was before we changed it" and "why did the report say that last month"
are unanswerable without both axes, and no amount of later engineering recovers
them. `recorded_at` is separate from `created_at` so a backfill can keep the
moment the system first learned the fact.

Full bitemporality is not free: every read grows two predicates. Add valid time
to tables where the world changes under you, and transaction time to tables
where your beliefs change. Most tables need neither; `created_at`/`updated_at`
is right for a user's display name.

On the two engines:

- **Timestamps.** Pass `utcnow()` values. SQLite drops the offset and stores the
  wall clock, so a non-UTC value is stored wrong
  ([portability.md](portability.md#timestamps)).
- **Partial indexes** work on both, SQLite uses `ix_fact_current` for
  `current()`, and Alembic autogenerate keeps both `*_where` arguments. Add
  `unique=True` on a subject column to refuse a second live row.
- **PostgreSQL only (not run):** range types (`tstzrange`) and exclusion
  constraints against overlapping periods. Keep the two columns above.

---

## Superseding, Not Overwriting

A correction closes the old row and opens a new one. It never edits in place.

```python
# app/models/fact.py
    def supersede(self, body: dict, source: str) -> 'Fact':
        """Close this fact and open its replacement. Call inside transaction()."""
        if self.source == 'user' and source != 'user':
            raise ValidationError('A human correction is not superseded automatically')

        now = utcnow()
        new = Fact(org_id=self.org_id, kind=self.kind, body=body, source=source,
                   valid_from=now, recorded_at=now)
        db.session.add(new)
        db.session.flush()                              # need new.id
        closed = db.session.execute(
            sa.update(Fact).where(Fact.id == self.id, Fact.valid_to.is_(None))
            .values(valid_to=now, superseded_by=new.id))
        if closed.rowcount == 0:
            raise ValidationError('Fact already superseded')   # lost the race; retry
        return new
```

Call it inside `with transaction():` ([database.md](database.md#transactions))
so both statements commit together and a refused supersede rolls back the row it
inserted. The `valid_to IS NULL` guard makes a concurrent double-supersede fail
rather than producing two live chains, on both engines. Load the fact through
the ORM first: the tenant filter covers reads, not this `UPDATE`.

Rules that hold across any domain doing this:

- **Record who superseded and why.** A `source` column (`'user'`, `'model'`,
  `'rule'`, `'import'`) is the minimum. Without it you cannot tell a human
  correction from an automated re-derivation.
- **Human corrections are sticky.** If a person corrected a fact, an automated
  process must not supersede it back. Otherwise the next reprocessing reasserts
  the original error and the user learns their correction is decorative.
- **Chains, not trees.** One live fact per subject. `superseded_by` forms a
  linked list you can walk backwards to reconstruct history.

---

## Entities, Edges, Evidence

When the domain is a set of things and relationships between them, not rows in
independent tables, three tables carry it:

```python
# app/models/graph.py
OPEN = sa.text('valid_to IS NULL')
UNMERGED = sa.text('canonical_id IS NULL')


class Entity(OrgScoped, BaseModel):
    """The things."""
    __tablename__ = 'entity'

    kind = db.Column(db.String(30), nullable=False)    # person | project | topic | resource
    canonical_id = db.Column(BigIntFK, db.ForeignKey('entity.id'), nullable=True)  # set when merged
    name = db.Column(db.String(200), nullable=False)
    attrs = db.Column(JSONColumn, nullable=False, default=dict)

    __table_args__ = (db.Index('ix_entity_kind', 'org_id', 'kind',
                               sqlite_where=UNMERGED, postgresql_where=UNMERGED),)


class Edge(OrgScoped, BaseModel):
    """The relationships. Typed, directed, temporal."""
    __tablename__ = 'edge'

    src_id = db.Column(BigIntFK, db.ForeignKey('entity.id'), nullable=False)
    dst_id = db.Column(BigIntFK, db.ForeignKey('entity.id'), nullable=False)
    kind = db.Column(db.String(30), nullable=False)    # owns | requested_from | affects
    attrs = db.Column(JSONColumn, nullable=False, default=dict)
    confidence = db.Column(db.Float, nullable=False, default=1.0)
    valid_from = db.Column(db.DateTime(timezone=True), nullable=False, default=utcnow)
    valid_to = db.Column(db.DateTime(timezone=True), nullable=True)
    superseded_by = db.Column(BigIntFK, db.ForeignKey('edge.id'), nullable=True)

    __table_args__ = (
        db.Index('ix_edge_out', 'org_id', 'src_id', 'kind',
                 sqlite_where=OPEN, postgresql_where=OPEN),
        db.Index('ix_edge_in', 'org_id', 'dst_id', 'kind',
                 sqlite_where=OPEN, postgresql_where=OPEN),
    )


class EdgeEvidence(OrgScoped, BaseModel):
    """Why you believe any of it."""
    __tablename__ = 'edge_evidence'

    edge_id = db.Column(BigIntFK, db.ForeignKey('edge.id', ondelete='CASCADE'),
                        nullable=False, index=True)
    message_id = db.Column(BigIntFK, db.ForeignKey('message.id', ondelete='CASCADE'),
                           nullable=False, index=True)

    __table_args__ = (
        db.UniqueConstraint('edge_id', 'message_id', name='uq_edge_evidence_edge_message'),
    )
```

Both directional indexes are required. Traversals go both ways ("what does Aidan
own", "who owns this"), and an index on one direction makes the other a scan of
the whole table.

**`attrs` is for attributes you only display. Anything you filter or join on
gets a column.** You cannot constrain a JSON key, cannot foreign-key it, and
cannot see it in a schema diff, and indexed JSON querying (JSONB with GIN) is
PostgreSQL only ([portability.md](portability.md#what-is-not-portable)). Start
narrow: promote a key to a column the first time you write a `where()` against
it. To change `attrs`, assign a new dict: an in-place
`entity.attrs['title'] = 'CTO'` is silently not saved.

---

## Identity Resolution

"Aidan", "@aidan", "Aidan K", and `aidan@example.com` are one person. Resolving
that is the hardest part of building a graph from unstructured input, and the
part people underestimate.

Two rules:

**Merge by pointer, never by deletion.** `canonical_id` marks an entity as
merged into another. Nothing is deleted, edges are not rewritten, and reads
follow the pointer. A wrong merge is then undone by clearing one pointer, rather
than being an unrecoverable loss.

```python
# app/models/graph.py  (method on Entity)
    @classmethod
    def resolved(cls):
        """Rows of (id, canonical id, canonical name) for every entity."""
        canon = aliased(cls)
        return db.session.execute(
            sa.select(cls.id,
                      sa.func.coalesce(canon.id, cls.id).label('canonical'),
                      sa.func.coalesce(canon.name, cls.name).label('name'))
            .outerjoin(canon, canon.id == cls.canonical_id)).all()
```

The join follows one pointer, so a merge keeps pointers one level deep: point
at the target's canonical entity and repoint anything already merged into the
source. This is a query rather than a database view because the tenant filter
scopes both sides of the self-join.

**Keep aliases as evidence.** An `EntityAlias` model (`entity_id`, `alias`,
`source`) lets you show why a match was made, and lets the next extraction
resolve the same surface form without re-deciding.

Bias toward under-merging. Two rows for one person is a visible annoyance a user
will point out. Two people merged into one silently attributes someone's work,
requests, and words to a colleague, and is discovered late and badly.

---

## Closed Vocabularies First

`kind` on entities and edges is an enum enforced in application code, and it is
a short list you wrote deliberately.

The temptation, especially with a model producing the data, is to let kinds be
free text so new concepts emerge on their own. What actually emerges is
`decision`, `Decision`, `decison`, `decision_made`, and `possible_decision`,
each with a handful of rows, and no query that returns all decisions.

```python
# app/models/graph.py
ENTITY_KINDS = frozenset({'person', 'project', 'topic', 'resource', 'request', 'decision'})


class Entity(OrgScoped, BaseModel):
    def validate(self):
        if self.kind not in ENTITY_KINDS:
            raise ValidationError(f'Unknown entity kind: {self.kind}')
```

Anything outside the set is rejected and counted: the model raises, and the
caller logs the kind ([models do not log](mvc.md#best-practices)). `validate()`
runs in `save()`, so a bulk `sa.insert()` must check the set itself. When the
rejection log shows the same unrecognised kind repeatedly, that is real evidence
for adding it: deliberately, with a display name and a query that uses it.

Open vocabularies are a feature to earn, not a starting position. Adding a kind
later is a one-line change; un-fragmenting a year of free-text kinds is a
migration you will not enjoy.

---

## Querying a Graph in Postgres

Everything here runs on SQLite as well; recursive CTEs are portable. One hop is
a join. Several hops is a recursive CTE:

```python
# app/models/graph.py  (method on Edge)
    @classmethod
    def reachable(cls, start_id: int, max_depth: int = 4) -> list[int]:
        """Ids of entities reachable from start_id over live edges."""
        hop = (sa.select(cls.dst_id.label('id'), sa.literal(1).label('depth'))
               .where(cls.src_id == start_id, cls.valid_to.is_(None))
               .cte('reachable', recursive=True))
        hop = hop.union(                                # union, not union_all
            sa.select(cls.dst_id, hop.c.depth + 1)
            .join(hop, cls.src_id == hop.c.id)
            .where(cls.valid_to.is_(None), hop.c.depth < max_depth))
        return list(db.session.scalars(sa.select(hop.c.id).distinct()))
```

Two non-negotiables: `union` rather than `union_all`, and an explicit depth cap.
The cap is what stops a cycle: each row carries its depth, so a loop yields a
new row at every level and `union` alone never ends it. `union` drops the
duplicates within a level. A graph built from real conversation is densely
connected, and a four-hop traversal without a cap will attempt to return most of
it. The tenant filter reaches inside the CTE, so another organization's edges
are not traversed.

In practice the useful queries are one and two hops ("what is attached to this
topic", "who is waiting on whom") and those are plain joins. If you find
yourself needing six-hop traversals to answer product questions, the question is
usually wrong before the database is.

---

## Deletion

Evidence and derivation delete differently, and a single "delete" verb across
both is a bug.

- **Derivation**: hard delete freely with `delete()`. It rebuilds from evidence.
- **Evidence**: soft delete (`deleted_at`), because something downstream cites it
  and a dangling citation is worse than a tombstone.
- **Legal erasure** (GDPR and similar): actually destroy the content, keep the
  row. Null the body, keep the id, timestamps, and structural links. Citations
  stay resolvable and render as "deleted", and the graph does not develop holes.

```python
# app/models/message.py  (evidence; body is nullable only so it can be erased)
    def erase(self):
        """Legal erasure: destroy the content, keep the row and its id."""
        self.body = None
        self.redacted = True
        self.deleted_at = self.deleted_at or utcnow()
        return self.save()
```

Removing a whole account is a different case: `ondelete='CASCADE'` on the owner
column takes evidence and derivation together, so nothing is left to dangle.

Decide this when you add the first evidence table. Retrofitting erasure onto a
schema where derived tables copied content rather than citing it means hunting
the same text through a dozen tables under a deadline.

---

## When You Need a Graph Database

Almost certainly not.

Both engines run the joins and the recursive CTE above with the indexes shown,
and when SQLite stops being enough the next step is PostgreSQL
([portability.md](portability.md#switching-engines)). A dedicated graph database
earns its place when traversal depth is genuinely unbounded and central to the
product (fraud rings, network topology, pathfinding), and it costs you
transactions with the rest of your data, a second operational system, and joins
to your relational tables becoming application-level work.

The realistic trigger is a specific recursive CTE that is too slow after
indexing and denormalisation, not the observation that the domain is
graph-shaped. Most graph-shaped domains have relational access patterns.

---

**Next:** [Database Patterns](database.md) | [Portability](portability.md) | [MVC Pattern](mvc.md)
