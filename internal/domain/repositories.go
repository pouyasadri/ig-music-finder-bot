package domain

import "context"

type UserRepository interface {
	Create(context.Context, *User) error
	GetByID(context.Context, int64) (*User, error)
	GetByTelegramID(context.Context, int64) (*User, error)
	Update(context.Context, *User) error
	Delete(context.Context, int64) error
}

type SettingsRepository interface {
	Get(context.Context, int64) (*UserSettings, error)
	Upsert(context.Context, *UserSettings) error
	Delete(context.Context, int64) error
}

type RequestRepository interface {
	Create(context.Context, *Request) error
	Update(context.Context, *Request) error
	GetByID(context.Context, int64) (*Request, error)
	GetByIDForUser(context.Context, int64, int64) (*Request, error)
	ListByUser(context.Context, int64, Page) ([]Request, error)
	Delete(context.Context, int64) error
	DeleteForUser(context.Context, int64, int64) error
}

type HistoryRepository = RequestRepository

type TrackRepository interface {
	Create(context.Context, *Track) error
	GetByID(context.Context, int64) (*Track, error)
	Delete(context.Context, int64) error
}

type FavoriteRepository interface {
	Add(context.Context, Favorite) error
	ListByUser(context.Context, int64, Page) ([]Favorite, error)
	Exists(context.Context, int64, int64) (bool, error)
	Delete(context.Context, int64, int64) error
	DeleteForUser(context.Context, int64) error
}

type PendingCallbackRepository interface {
	Create(context.Context, *PendingCallbackAction) error
	GetByIDForUser(context.Context, int64, int64) (*PendingCallbackAction, error)
	ListByUser(context.Context, int64, Page) ([]PendingCallbackAction, error)
	Delete(context.Context, int64, int64) error
	DeleteExpired(context.Context) error
}
