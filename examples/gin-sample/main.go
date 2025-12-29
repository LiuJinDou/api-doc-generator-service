package main

import (
	"github.com/gin-gonic/gin"
)

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func main() {
	r := gin.Default()

	// User routes
	r.GET("/users", ListUsers)
	r.GET("/users/:id", GetUser)
	r.POST("/users", CreateUser)
	r.PUT("/users/:id", UpdateUser)
	r.DELETE("/users/:id", DeleteUser)

	// Product routes
	r.GET("/products", ListProducts)
	r.GET("/products/:id", GetProduct)

	r.Run(":8080")
}

func ListUsers(c *gin.Context) {
	users := []User{
		{ID: 1, Name: "Alice", Email: "alice@example.com"},
		{ID: 2, Name: "Bob", Email: "bob@example.com"},
	}
	c.JSON(200, users)
}

func GetUser(c *gin.Context) {
	id := c.Param("id")
	user := User{ID: 1, Name: "Alice", Email: "alice@example.com"}
	c.JSON(200, gin.H{"id": id, "user": user})
}

func CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	user := User{ID: 3, Name: req.Name, Email: req.Email}
	c.JSON(201, user)
}

func UpdateUser(c *gin.Context) {
	id := c.Param("id")
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"id": id, "updated": true})
}

func DeleteUser(c *gin.Context) {
	id := c.Param("id")
	c.JSON(200, gin.H{"id": id, "deleted": true})
}

func ListProducts(c *gin.Context) {
	c.JSON(200, gin.H{"products": []string{"Product1", "Product2"}})
}

func GetProduct(c *gin.Context) {
	id := c.Param("id")
	c.JSON(200, gin.H{"id": id, "name": "Product"})
}
