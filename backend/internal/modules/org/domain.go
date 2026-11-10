// Package org owns departments, categories and routing rules (admin configuration).
package org

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"modusvoice/internal/platform/apperr"
)

// Department is a flat org unit with an optional head.
type Department struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	HeadUserID *int64  `json:"head_user_id"`
	HeadName   *string `json:"head_name"`
	HeadActive bool    `json:"-"`
}

// Category is a request category shown on the submit form.
type Category struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Sort        int    `json:"sort"`
	Active      bool   `json:"active"`
}

// Rule maps (category, type) to a department, default assignee and SLA hours.
type Rule struct {
	ID                 int64   `json:"id"`
	CategoryID         int64   `json:"category_id"`
	CategoryName       string  `json:"category_name"`
	RequestType        *string `json:"request_type"`
	DepartmentID       int64   `json:"department_id"`
	DepartmentName     string  `json:"department_name"`
	AssigneeUserID     *int64  `json:"assignee_user_id"`
	AssigneeName       *string `json:"assignee_name"`
	FirstResponseHours int     `json:"first_response_hours"`
	ResolutionHours    int     `json:"resolution_hours"`
	Active             bool    `json:"active"`
}

// CategoryInput is the admin create/patch payload (nil = not provided).
type CategoryInput struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Icon        *string `json:"icon"`
	Sort        *int    `json:"sort"`
	Active      *bool   `json:"active"`
}

var iconRe = regexp.MustCompile(`^[a-z0-9-]{1,40}$`)

// apply validates and applies in onto c.
func (in CategoryInput) apply(c *Category, create bool) error {
	var v apperr.V
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if l := utf8.RuneCountInString(n); l < 2 || l > 80 {
			v.Add("name", "Название: от 2 до 80 символов")
		}
		c.Name = n
	} else if create {
		v.Add("name", "Укажите название")
	}
	if in.Description != nil {
		d := strings.TrimSpace(*in.Description)
		if utf8.RuneCountInString(d) > 500 {
			v.Add("description", "Описание: не более 500 символов")
		}
		c.Description = d
	}
	if in.Icon != nil {
		if !iconRe.MatchString(*in.Icon) {
			v.Add("icon", "Иконка: имя иконки lucide (латиница, цифры, дефис)")
		}
		c.Icon = *in.Icon
	}
	if in.Sort != nil {
		if *in.Sort < -10000 || *in.Sort > 10000 {
			v.Add("sort", "Порядок: от -10000 до 10000")
		}
		c.Sort = *in.Sort
	}
	if in.Active != nil {
		c.Active = *in.Active
	}
	return v.Err()
}

// ValidRequestType reports whether t is a known request type.
func ValidRequestType(t string) bool { return t == "problem" || t == "idea" }

const maxSLAHours = 8760
