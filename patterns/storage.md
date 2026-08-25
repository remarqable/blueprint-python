# File Storage & Uploads

> **Layer doc.** Read this only when `uploads: true` in the Project
> Configuration. Pairs with [tenancy.md](tenancy.md) when `tenancy: shared` —
> uploaded files belong to a tenant like any other row.

Logos, avatars, featured images, attachments. Local disk behind a small
interface, so production can move to S3-compatible storage later by swapping
one class — never by touching call sites.

---

## Table of Contents

- [Principles](#principles)
- [The Interface](#the-interface)
- [Local Backend](#local-backend)
- [The Upload Model](#the-upload-model)
- [Accepting Uploads](#accepting-uploads)
- [Images](#images)
- [Serving Files](#serving-files)
- [Security](#security)

---

## Principles

- **Files live on the data volume** (`$DATA_DIR/uploads/`), never inside
  `app/static/`. Static is code, shipped with deploys; uploads are data,
  owned by the installation.
- **The stored name is never the uploaded name.** Generate a random key;
  keep the original filename as metadata in the database.
- **Every file has a row.** The `Upload` model is the source of truth —
  org ownership, visibility, content type, size. A file with no row does
  not exist; a row whose file is missing is an integrity error worth logging.
- **One interface, many backends.** Call sites use `storage.save/open/delete/
  url`; whether that hits local disk or S3 is configuration.

---

## The Interface

```python
# app/platform/storage.py
from typing import BinaryIO, Protocol


class Storage(Protocol):
    def save(self, key: str, stream: BinaryIO) -> None: ...
    def open(self, key: str) -> BinaryIO: ...
    def delete(self, key: str) -> None: ...
    def exists(self, key: str) -> bool: ...
```

Keys are relative POSIX-style paths chosen by the application, e.g.
`org/42/2026/8f3c9a…e1.webp`. Prefixing with the org id makes per-tenant
export and purge a directory operation.

---

## Local Backend

```python
# app/platform/storage.py
from pathlib import Path


class LocalStorage:
    def __init__(self, root: Path):
        self.root = root

    def _path(self, key: str) -> Path:
        p = (self.root / key).resolve()
        if not p.is_relative_to(self.root.resolve()):
            raise ValueError(f'Unsafe storage key: {key}')
        return p

    def save(self, key: str, stream) -> None:
        p = self._path(key)
        p.parent.mkdir(parents=True, exist_ok=True)
        with open(p, 'wb') as f:
            for chunk in iter(lambda: stream.read(65536), b''):
                f.write(chunk)

    def open(self, key: str):
        return open(self._path(key), 'rb')

    def delete(self, key: str) -> None:
        self._path(key).unlink(missing_ok=True)

    def exists(self, key: str) -> bool:
        return self._path(key).is_file()


def init_storage(app):
    root = Path(app.config['DATA_DIR']) / 'uploads'
    app.extensions['storage'] = LocalStorage(root)
```

The `_path` check is not optional. Keys normally come from your own database,
but defence in depth costs two lines and a traversal via a corrupted key
costs the server.

---

## The Upload Model

```python
# app/models/upload.py
class Upload(OrgScoped, BaseModel):          # tenancy: shared
    __tablename__ = 'upload'

    key = db.Column(db.String(255), unique=True, nullable=False)
    filename = db.Column(db.String(255), nullable=False)     # original, display only
    content_type = db.Column(db.String(100), nullable=False)
    size = db.Column(db.Integer, nullable=False)
    visibility = db.Column(db.String(10), nullable=False, default='private')
    # 'public'  -> anyone with the URL
    # 'private' -> members of the owning org only
    created_by = db.Column(BigIntFK, db.ForeignKey('user.id'), nullable=True)
```

With `tenancy: personal`, replace `OrgScoped` with the user-owned pattern from
[core/database.md](core/database.md#user-owned-data).

---

## Accepting Uploads

```python
# app/models/upload.py
import secrets
from pathlib import PurePosixPath

ALLOWED = {
    'image/png': '.png', 'image/jpeg': '.jpg',
    'image/webp': '.webp', 'image/gif': '.gif',
    'application/pdf': '.pdf',
}
MAX_SIZE = 10 * 1024 * 1024        # also set MAX_CONTENT_LENGTH in config


@classmethod
def from_file(cls, file, visibility='private') -> 'Upload':
    """file is a werkzeug FileStorage from request.files."""
    head = file.stream.read(MAX_SIZE + 1)
    if len(head) > MAX_SIZE:
        raise ValidationError('File too large')

    content_type = _sniff(head)                 # from bytes, never the client header
    if content_type not in ALLOWED:
        raise ValidationError('File type not allowed')

    key = f'org/{g.org.id}/{secrets.token_hex(16)}{ALLOWED[content_type]}'
    storage().save(key, io.BytesIO(head))

    return cls(key=key, filename=file.filename or 'upload',
               content_type=content_type, size=len(head),
               visibility=visibility).save()
```

Sniff the type from the bytes (`filetype` package, or Pillow for images —
opening the image *is* the sniff). The client's `Content-Type` header and the
filename extension are both attacker-controlled.

---

## Images

Resize on upload, not on request. Store the variants you actually render:

```python
VARIANTS = {'thumb': 200, 'medium': 800, 'full': 1600}   # max long edge, px

def make_variants(upload: Upload) -> None:
    from PIL import Image
    with storage().open(upload.key) as f:
        img = Image.open(f)
        img = ImageOps.exif_transpose(img)      # honour rotation, then...
        img.info.pop('exif', None)              # ...strip EXIF (GPS!) on save
        for name, edge in VARIANTS.items():
            copy = img.copy()
            copy.thumbnail((edge, edge))
            out = io.BytesIO()
            copy.save(out, 'WEBP', quality=82)
            out.seek(0)
            storage().save(variant_key(upload.key, name), out)
```

- Re-encoding to WebP (or JPEG) is also the sanitizer: whatever was hiding in
  the original container does not survive a decode/re-encode.
- Strip EXIF. Uploaded phone photos carry GPS coordinates; republishing them
  is a privacy incident.
- Set `Image.MAX_IMAGE_PIXELS` (Pillow's decompression-bomb guard) rather
  than disabling the warning.

---

## Serving Files

One route, visibility enforced, no filesystem paths in URLs:

```python
@bp.route('/files/<int:upload_id>/<variant>')
def serve_upload(upload_id: int, variant: str):
    upload = db.session.get(Upload, upload_id)   # tenant filter applies
    if upload is None:
        abort(404)
    if upload.visibility != 'public' and g.membership is None:
        abort(404)
    if variant not in VARIANTS and variant != 'original':
        abort(404)

    return send_file(storage().open(key_for(upload, variant)),
                     mimetype=upload.content_type, max_age=31536000,
                     download_name=upload.filename)
```

Because the lookup is by id through the ORM, the tenant filter from
[tenancy.md](tenancy.md) already prevents cross-tenant access — the same IDOR
protection as every other model. Public files on busy installations can later
move behind the reverse proxy or a CDN without changing the URL scheme.

---

## Security

- [x] Type sniffed from bytes; client headers and extensions ignored
- [x] Random storage keys; original filename is metadata only
- [x] Size limit enforced in code **and** `MAX_CONTENT_LENGTH` in config
- [x] No SVG in the allowed list — SVG is XML with script; if you must accept
      it, sanitize server-side and serve with `Content-Security-Policy: default-src 'none'`
- [x] `X-Content-Type-Options: nosniff` on served files
- [x] EXIF stripped from images; re-encode as sanitizer
- [x] Uploads directory outside the app tree, on the data volume, excluded
      from git
- [x] Purge files when their row is deleted (and in the tenant-deletion job —
      see [tenancy.md § Tenant Lifecycle](tenancy.md#tenant-lifecycle))

---

**Next:** [Jobs](jobs.md) | [Tenancy](tenancy.md) | [Security](core/security.md)
