package storage

import (
	"context"
	"testing"
	"time"

	"github.com/calvinmclean/babyapi"
	"github.com/madflojo/hord/drivers/hashmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TODO struct {
	babyapi.DefaultResource

	Title       string
	Description string
	Completed   bool
}

func TestClient(t *testing.T) {
	db, err := NewFileDB(hashmap.Config{})
	assert.NoError(t, err)
	c := NewClient[*TODO](db, "TODO")

	id := babyapi.NewID()
	t.Run("StoreTODO", func(t *testing.T) {
		err := c.Set(context.Background(), &TODO{DefaultResource: babyapi.DefaultResource{ID: id}, Title: "TODO 1"})
		require.NoError(t, err)
	})
	t.Run("GetTODO", func(t *testing.T) {
		todo, err := c.Get(context.Background(), id.String())
		require.NoError(t, err)
		require.Equal(t, "TODO 1", todo.Title)
	})
	t.Run("GetAllTODOs", func(t *testing.T) {
		todos, err := c.GetAll(context.Background(), func(t *TODO) bool { return true })
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
		require.ErrorIs(t, err, babyapi.ErrNotFound)
	})
	t.Run("DeleteTODONotFound", func(t *testing.T) {
		err := c.Delete(context.Background(), id.String())
		require.Error(t, err)
		require.ErrorIs(t, err, babyapi.ErrNotFound)
	})
}

type EndDateableTODO struct {
	babyapi.DefaultResource

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
	db, err := NewFileDB(hashmap.Config{})
	assert.NoError(t, err)
	c := NewClient[*EndDateableTODO](db, "TODO")

	id := babyapi.NewID()

	t.Run("StoreTODO", func(t *testing.T) {
		err := c.Set(context.Background(), &EndDateableTODO{DefaultResource: babyapi.DefaultResource{ID: id}, Title: "TODO 1"})
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
	t.Run("HardDeleteTODO", func(t *testing.T) {
		err := c.Delete(context.Background(), id.String())
		require.NoError(t, err)
	})
	t.Run("GetTODONotFound", func(t *testing.T) {
		_, err := c.Get(context.Background(), id.String())
		require.Error(t, err)
		require.ErrorIs(t, err, babyapi.ErrNotFound)
	})
}
