package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Store holds everything in memory. Restarting or POST /mock/reset reseeds it.
type Store struct {
	mu       sync.RWMutex
	now      func() time.Time
	banks    []Bank
	users    []User
	accounts []*Account
	txs      []*Transaction
	consents map[string]*Consent
	nextSeq  int
}

func NewStore(now func() time.Time) *Store {
	s := &Store{now: now}
	s.Reset()
	return s
}

func (s *Store) Reset() {
	users, accounts, txs := seedData(s.now())
	s.mu.Lock()
	defer s.mu.Unlock()
	s.banks = seedBanks
	s.users = users
	s.accounts = accounts
	s.txs = txs
	s.consents = map[string]*Consent{}
	s.nextSeq = len(txs) + 1
}

func (s *Store) today() time.Time { return dateOf(s.now()) }

func (s *Store) bank(code string) (Bank, bool) {
	for _, b := range s.banks {
		if b.Code == code {
			return b, true
		}
	}
	return Bank{}, false
}

func (s *Store) user(psuID string) (User, bool) {
	for _, u := range s.users {
		if u.PSUID == psuID {
			return u, true
		}
	}
	return User{}, false
}

func (s *Store) account(id string) *Account {
	for _, a := range s.accounts {
		if a.ResourceID == id {
			return a
		}
	}
	return nil
}

// consentAccounts returns the accounts one consent covers: one user at one bank.
func (s *Store) consentAccounts(c *Consent) []*Account {
	var out []*Account
	for _, a := range s.accounts {
		if a.PSUID == c.PSUID && a.BankCode == c.BankCode {
			out = append(out, a)
		}
	}
	return out
}

// refreshStatus moves a valid consent past its validUntil date to expired.
// Callers hold the write lock.
func (s *Store) refreshStatus(c *Consent) {
	if c.Status == StatusValid && s.today().After(c.ValidUntil) {
		c.Status = StatusExpired
	}
}

func newConsentID(taken map[string]*Consent) string {
	for {
		b := make([]byte, 3)
		_, _ = rand.Read(b)
		id := "cns-" + hex.EncodeToString(b)
		if _, ok := taken[id]; !ok {
			return id
		}
	}
}

type balances struct {
	interimAvailable Money // booked + pending, i.e. what can be spent now
	closingBooked    Money // booked balance at the end of yesterday
}

func (s *Store) balances(a *Account) balances {
	today := s.today()
	booked, todayBooked, pending := a.Opening, Money(0), Money(0)
	for _, t := range s.txs {
		if t.AccountID != a.ResourceID {
			continue
		}
		switch {
		case !t.Booked:
			pending += t.Amount
		case t.Date.Equal(today):
			todayBooked += t.Amount
			booked += t.Amount
		default:
			booked += t.Amount
		}
	}
	return balances{interimAvailable: booked + pending, closingBooked: booked - todayBooked}
}

// transactions returns booked entries in [from, to] and all pending ones, newest first.
func (s *Store) transactions(a *Account, from, to time.Time) (booked, pending []*Transaction) {
	for _, t := range s.txs {
		if t.AccountID != a.ResourceID {
			continue
		}
		if !t.Booked {
			pending = append(pending, t)
		} else if !t.Date.Before(from) && !t.Date.After(to) {
			booked = append(booked, t)
		}
	}
	newestFirst := func(xs []*Transaction) {
		sort.Slice(xs, func(i, j int) bool {
			if !xs[i].Date.Equal(xs[j].Date) {
				return xs[i].Date.After(xs[j].Date)
			}
			return xs[i].seq > xs[j].seq
		})
	}
	newestFirst(booked)
	newestFirst(pending)
	return booked, pending
}

func (s *Store) inject(t *Transaction) {
	t.seq = s.nextSeq
	t.ID = fmt.Sprintf("tx-%06d", s.nextSeq)
	s.nextSeq++
	s.txs = append(s.txs, t)
}
