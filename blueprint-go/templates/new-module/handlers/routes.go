package handlers

import (
	"net/http"
	"strconv"

	"yourapp/internal/modules/modulename/models"
	"yourapp/internal/system/auth"

	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests for this module
type Handler struct {
	itemRepo *models.ItemRepository
}

// NewHandler creates a new handler
func NewHandler(repo *models.ItemRepository) *Handler {
	return &Handler{itemRepo: repo}
}

// RegisterRoutes registers all routes for this module
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	// Apply auth middleware to all routes
	r.Use(auth.RequireLogin())

	r.GET("/", h.Index)
	r.GET("/:id", h.Show)
	r.POST("/", h.Create)
	r.PUT("/:id", h.Update)
	r.DELETE("/:id", h.Delete)
}

// Index lists all items
func (h *Handler) Index(c *gin.Context) {
	items, err := h.itemRepo.GetAll()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "errors/500.html", gin.H{
			"error": "Failed to load items",
		})
		return
	}

	// Return partial for HTMX requests
	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusOK, "modulename/partials/_list.html", gin.H{
			"items": items,
		})
		return
	}

	c.HTML(http.StatusOK, "modulename/index.html", gin.H{
		"items": items,
		"title": "Items",
	})
}

// Show displays a single item
func (h *Handler) Show(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.HTML(http.StatusBadRequest, "errors/400.html", gin.H{
			"error": "Invalid ID",
		})
		return
	}

	item, err := h.itemRepo.GetByID(uint(id))
	if err != nil {
		c.HTML(http.StatusNotFound, "errors/404.html", gin.H{
			"error": "Item not found",
		})
		return
	}

	c.HTML(http.StatusOK, "modulename/show.html", gin.H{
		"item":  item,
		"title": item.Name,
	})
}

// Create handles item creation
func (h *Handler) Create(c *gin.Context) {
	name := c.PostForm("name")
	description := c.PostForm("description")

	item := &models.Item{
		Name:        name,
		Description: description,
	}

	if err := h.itemRepo.Create(item); err != nil {
		// Return error for HTMX
		if c.GetHeader("HX-Request") == "true" {
			c.HTML(http.StatusBadRequest, "modulename/partials/_form_error.html", gin.H{
				"error": err.Error(),
			})
			return
		}

		c.HTML(http.StatusBadRequest, "modulename/new.html", gin.H{
			"error": err.Error(),
			"item":  item,
		})
		return
	}

	// HTMX: return new item partial
	if c.GetHeader("HX-Request") == "true" {
		c.Header("HX-Trigger", `{"showToast": {"message": "Item created!", "type": "success"}}`)
		c.HTML(http.StatusCreated, "modulename/partials/_item.html", gin.H{
			"item": item,
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/modulename")
}

// Update handles item updates
func (h *Handler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	item, err := h.itemRepo.GetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	item.Name = c.PostForm("name")
	item.Description = c.PostForm("description")

	if err := h.itemRepo.Update(item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// HTMX: return updated item
	if c.GetHeader("HX-Request") == "true" {
		c.Header("HX-Trigger", `{"showToast": {"message": "Item updated!", "type": "success"}}`)
		c.HTML(http.StatusOK, "modulename/partials/_item.html", gin.H{
			"item": item,
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/modulename/"+c.Param("id"))
}

// Delete handles item deletion
func (h *Handler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	if err := h.itemRepo.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete"})
		return
	}

	// HTMX: trigger event and return empty (item will be removed from DOM)
	if c.GetHeader("HX-Request") == "true" {
		c.Header("HX-Trigger", `{"showToast": {"message": "Item deleted", "type": "info"}}`)
		c.Status(http.StatusOK)
		return
	}

	c.Redirect(http.StatusSeeOther, "/modulename")
}
