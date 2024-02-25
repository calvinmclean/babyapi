package babyapi

import "fmt"

// Extension is a way that a collection of modifiers, or other code, can be applied to an API all at once. This
// makes code more reusable and allows external libraries to provide modifiers
type Extension[T Resource] interface {
	Apply(*API[T]) error
}

// ApplyExtension adds an Extension to the API and applies it
func (a *API[T]) ApplyExtension(e Extension[T]) *API[T] {
	a.panicIfReadOnly()

	err := e.Apply(a)
	if err != nil {
		// TODO: when #32 is implemented, extensions will be added to a list and applied in the function
		// that finalizes the API and returns an error
		panic(fmt.Sprintf("error applying extension: %v", err))
	}
	return a
}
