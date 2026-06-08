package utils

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Role string

const (
	RoleOwner Role = "owner"

// RoleFriend Role = "friend"
// RoleAgent  Role = "agent"
)

type Connection struct {
	Provider string         `json:"provider"`
	Data     map[string]any `json:"data"`
}

type User struct {
	Id          int          `json:"id"`
	Name        string       `json:"name"`
	Role        Role         `json:"role"`
	Connections []Connection `json:"connections"`
}

type UsersFile struct {
	NextId int    `json:"next_id"`
	Users  []User `json:"users"`
}
type UserStore struct {
	path string
	data UsersFile
}

func (u *User) MakeJson() (string, error) {
	b, err := json.MarshalIndent(
		u,
		"",
		"\t",
	)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func CreateUserStore(path string) (*UserStore, *TraceError) {

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, Wrap(err)
	}

	_, err := os.Stat(path)
	if err == nil {
		bytes, err := os.ReadFile(path)

		if err == nil {

			var data UsersFile

			if err := json.Unmarshal(
				bytes,
				&data,
			); err != nil {
				return nil, Wrap(err)
			}

			return &UserStore{
				path: path,
				data: data,
			}, nil

		} else {
			return nil, Wrap(err)
		}

	} else if errors.Is(err, os.ErrNotExist) {

		data := UsersFile{
			NextId: 0,
			Users:  []User{},
		}
		userStore := UserStore{
			path: path,
			data: data,
		}

		if err := userStore.save(); err != nil {
			return nil, err
		}

		return &userStore, nil

	} else {
		return nil, Wrap(err)
	}
}

func (s *UserStore) save() *TraceError {

	bytes, err := json.MarshalIndent(
		s.data,
		"",
		"\t",
	)

	if err != nil {
		return Wrap(err)
	}

	return Wrap(os.WriteFile(
		s.path,
		bytes,
		0644,
	))
}

func (s *UserStore) OwnerExists() (
	bool,
	error,
) {

	for _, user := range s.data.Users {
		if user.Role == RoleOwner {
			return true, nil
		}
	}

	return false, nil
}

func getNextId(users []User) int {

	candidateId := len(users)
	for {
		startCandiateId := candidateId
		for _, user := range users {
			if user.Id == candidateId {
				candidateId = user.Id + 1
			}
		}
		if startCandiateId == candidateId {
			break
		}
	}
	return candidateId
}

func (s *UserStore) CreateOwner(
	name string,
	telegramId int64,
	chatId int64,
) *TraceError {

	id := s.data.NextId
	s.data.NextId++

	s.data.Users = append(
		s.data.Users,
		User{
			Id:   id,
			Name: name,
			Role: RoleOwner,
			Connections: []Connection{
				{
					Provider: "telegram",
					Data: map[string]any{
						"user_id": telegramId,
						"chat_id": chatId,
					},
				},
			},
		},
	)

	return s.save()
}

func (s *UserStore) GetTelegramUser(id int64) (*User, error) {

	for _, user := range s.data.Users {
		for _, connection := range user.Connections {
			if connection.Provider == "telegram" {
				if toInt64(connection.Data["user_id"]) == toInt64(id) {
					return &user, nil
				}
			}
		}
	}

	return nil, nil
}

func toInt64(val interface{}) int64 {
	switch v := val.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	}
	return 0
}
