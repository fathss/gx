package cli

type Error struct {
	Message string
	Hint    string
}

func (e *Error) Error() string {
	return e.Message
}