package signaler

import (
	"fmt"
	"sync"
	"time"

	"github.com/lainio/err2"
	"github.com/lainio/err2/try"
)

type UserScopes struct {
	mu         *sync.RWMutex
	userScopes map[string]*UserScope

	ScopeSurvivalTime  time.Duration
	ScopeCheckInterval time.Duration
}

func NewUserScopes() *UserScopes {
	return &UserScopes{
		mu:         &sync.RWMutex{},
		userScopes: map[string]*UserScope{},

		ScopeSurvivalTime:  5 * time.Minute,
		ScopeCheckInterval: time.Second,
	}
}

func (scopes *UserScopes) Get(auth UserAuth) (s *UserScope, err error) {
	defer err2.Return(&err)
	defer func() { // keep scope alive
		if s != nil {
			s.KeepAlive()
		}
	}()

	user, pass := auth.Username, auth.Password

	if user == "" {
		return nil, fmt.Errorf("username is not defined")
	}

	scopes.mu.RLock()
	s, ok := scopes.userScopes[user]
	scopes.mu.RUnlock()

	if ok {
		try.To(
			s.CheckPassword(pass))

		return
	}

	s = try.To1(
		scopes.Create(auth))
	go scopes.DeleteAfter(s, scopes.ScopeSurvivalTime)

	return
}

func (scopes *UserScopes) Create(auth UserAuth) (s *UserScope, err error) {
	scopes.mu.Lock()
	defer scopes.mu.Unlock()

	s = try.To1(
		NewUserScope(auth))

	user := s.Auth.Username
	scopes.userScopes[user] = s

	return
}

func (scopes *UserScopes) DeleteAfter(scope *UserScope, d time.Duration) {
	for {
		time.Sleep(scopes.ScopeCheckInterval)
		now := time.Now()
		expiresAt := scope.LastAliveAt.Add(d)
		if expiresAt.Before(now) {
			scopes.Delete(scope.Auth.Username)
			break
		}
	}
}

func (scopes *UserScopes) Delete(user string) {
	scopes.mu.Lock()
	defer scopes.mu.Unlock()
	_, ok := scopes.userScopes[user]
	if !ok { //deleted
		return
	}
	delete(scopes.userScopes, user)
}
