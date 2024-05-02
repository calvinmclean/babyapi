package babyapi

import (
	"context"
	"testing"
	"time"

	"github.com/calvinmclean/babyapi/storage/kv"
	"github.com/madflojo/hord/drivers/hashmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TODO struct {
	DefaultResource

	Title       string
	Description string
	Completed   bool
}

func TestClient(t *testing.T) {
	db, err := kv.NewFileDB(hashmap.Config{})
	assert.NoError(t, err)
	c := NewKVStorage[*TODO](db, "TODO")

	id := NewID()
	t.Run("StoreTODO", func(t *testing.T) {
		err := c.Set(context.Background(), &TODO{DefaultResource: DefaultResource{ID: id}, Title: "TODO 1"})
		require.NoError(t, err)
	})
	t.Run("GetTODO", func(t *testing.T) {
		todo, err := c.Get(context.Background(), id.String())
		require.NoError(t, err)
		require.Equal(t, "TODO 1", todo.Title)
	})
	t.Run("GetAllTODOs", func(t *testing.T) {
		todos, err := c.GetAll(context.Background(), nil)
		require.NoError(t, err)
		require.Len(t, todos, 1)
		require.Equal(t, "TODO 1", todos[0].Title)
	})
	t.Run("DeleteTODO", func(t *testing.T) {
		err := c.Delete(context.Background(), id.String())
		require.NoError(t, err)
	})
	t.Run("GetTODONotFound", func(t *testing.T) {
		_, err := c.Get(context.Background(), id.String())
		require.Error(t, err)
		require.ErrorIs(t, err, ErrNotFound)
	})
	t.Run("DeleteTODONotFound", func(t *testing.T) {
		err := c.Delete(context.Background(), id.String())
		require.Error(t, err)
		require.ErrorIs(t, err, ErrNotFound)
	})
	t.Run("GetAllTODOsAgainWithEndDatedDefaultFalse", func(t *testing.T) {
		todos, err := c.GetAll(context.Background(), nil)
		require.NoError(t, err)
		require.Empty(t, todos)
	})
	t.Run("GetAllTODOsAgainWithEndDatedFalse", func(t *testing.T) {
		todos, err := c.GetAll(context.Background(), EndDatedQueryParam(false))
		require.NoError(t, err)
		require.Empty(t, todos)
	})
	t.Run("GetAllTODOsWithEndDatedTrueStillShowsNone", func(t *testing.T) {
		todos, err := c.GetAll(context.Background(), EndDatedQueryParam(true))
		require.NoError(t, err)
		require.Empty(t, todos)
	})
}

type EndDateableTODO struct {
	DefaultResource

	Title       string
	Description string
	Completed   bool
	EndDate     *time.Time
}

func (t *EndDateableTODO) EndDated() bool {
	return t.EndDate != nil && t.EndDate.Before(time.Now())
}

func (t *EndDateableTODO) SetEndDate(now time.Time) {
	t.EndDate = &now
}

func TestEndDateable(t *testing.T) {
	db, err := kv.NewFileDB(hashmap.Config{})
	assert.NoError(t, err)
	c := NewKVStorage[*EndDateableTODO](db, "TODO")

	id := NewID()

	t.Run("StoreTODO", func(t *testing.T) {
		err := c.Set(context.Background(), &EndDateableTODO{DefaultResource: DefaultResource{ID: id}, Title: "TODO 1"})
		require.NoError(t, err)
	})
	t.Run("GetTODOIsNotEndDated", func(t *testing.T) {
		todo, err := c.Get(context.Background(), id.String())
		require.NoError(t, err)
		require.False(t, todo.EndDated())
	})
	t.Run("SoftDeleteTODO", func(t *testing.T) {
		err := c.Delete(context.Background(), id.String())
		require.NoError(t, err)
	})
	t.Run("GetTODOHasEndDate", func(t *testing.T) {
		todo, err := c.Get(context.Background(), id.String())
		require.NoError(t, err)
		require.True(t, todo.EndDated())
	})
	t.Run("GetAllTODOsAgainWithEndDatedDefaultFalse", func(t *testing.T) {
		todos, err := c.GetAll(context.Background(), nil)
		require.NoError(t, err)
		require.Empty(t, todos)
	})
	t.Run("GetAllTODOsAgainWithEndDatedFalse", func(t *testing.T) {
		todos, err := c.GetAll(context.Background(), EndDatedQueryParam(false))
		require.NoError(t, err)
		require.Empty(t, todos)
	})
	t.Run("GetAllTODOsWithEndDatedTrue", func(t *testing.T) {
		todos, err := c.GetAll(context.Background(), EndDatedQueryParam(true))
		require.NoError(t, err)
		require.Len(t, todos, 1)
		require.Equal(t, "TODO 1", todos[0].Title)
	})
	t.Run("HardDeleteTODO", func(t *testing.T) {
		err := c.Delete(context.Background(), id.String())
		require.NoError(t, err)
	})
	t.Run("GetTODONotFound", func(t *testing.T) {
		_, err := c.Get(context.Background(), id.String())
		require.Error(t, err)
		require.ErrorIs(t, err, ErrNotFound)
	})
}
