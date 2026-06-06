package utils

import "testing"

func TestGetNextId(t *testing.T) {
	testCases := []struct {
		name     string
		users    []User
		expected int
	}{
		{
			name: "empty list",
			users: []User{
				{
					Id:   0,
					Name: "test",
				},
			},
			expected: 1,
		},
		{
			name: "list with one item",
			users: []User{
				{
					Id:   0,
					Name: "test",
				},
			},
			expected: 1,
		},
		{
			name: "list with one item conflict",
			users: []User{
				{
					Id:   1,
					Name: "test",
				},
			},
			expected: 2,
		},
		{
			name: "list with two items",
			users: []User{
				{
					Id:   0,
					Name: "test",
				},
				{
					Id:   1,
					Name: "test",
				},
			},
			expected: 2,
		},
		{
			name: "list with two items conflict",
			users: []User{
				{
					Id:   2,
					Name: "test",
				},
				{
					Id:   3,
					Name: "test",
				},
			},
			expected: 4,
		},
		{
			name: "list with three items",
			users: []User{
				{
					Id:   0,
					Name: "test",
				},
				{
					Id:   1,
					Name: "test",
				},
				{
					Id:   2,
					Name: "test",
				},
			},
			expected: 3,
		},
		{
			name: "list with three items conflict",
			users: []User{
				{
					Id:   3,
					Name: "test",
				},
				{
					Id:   4,
					Name: "test",
				},
				{
					Id:   6,
					Name: "test",
				},
			},
			expected: 5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if getNextId(tc.users) != tc.expected {
				t.Errorf("Expected %d, got %d", tc.expected, getNextId(tc.users))
			}
		})
	}
}
