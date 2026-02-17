//go:build ignore

package sample

import (
	"fmt"
	"strings"

	myalias "github.com/go-chi/chi/v5"
)

// User represents a person in the system.
type User struct {
	Name string
	Age  int
}

// Admin has elevated privileges.
type Admin struct {
	User
	Role string
}

// Serializer defines serialization behavior.
type Serializer interface {
	Serialize() string
}

// UserID is a unique identifier for users.
type UserID string

// NewUser creates a new User.
func NewUser(name string, age int) *User {
	return &User{Name: name, Age: age}
}

// Greet returns a greeting string.
func (u *User) Greet() string {
	return fmt.Sprintf("Hello, %s", u.Name)
}

func (u User) String() string {
	return strings.Join([]string{u.Name}, ", ")
}

func (a *Admin) Promote(target *User) {
	fmt.Println("Promoting", target.Name)
	myalias.NewRouter()
}

func process(users []*User) {
	for _, u := range users {
		fmt.Println(u.Greet())
	}
}
