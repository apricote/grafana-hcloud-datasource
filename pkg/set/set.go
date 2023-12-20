package set

type Set[T comparable] map[T]struct{}

func New[T comparable]() Set[T] {
	return make(Set[T])
}

func (s Set[T]) Insert(elements ...T) {
	for _, element := range elements {
		s[element] = struct{}{}
	}
}

func (s Set[T]) Has(element T) bool {
	_, ok := s[element]
	return ok
}

func (s Set[T]) ToSlice() []T {
	elements := make([]T, 0, len(s))

	for element := range s {
		elements = append(elements, element)
	}

	return elements
}

func From[T comparable](element ...T) Set[T] {
	set := New[T]()
	set.Insert(element...)
	return set
}
