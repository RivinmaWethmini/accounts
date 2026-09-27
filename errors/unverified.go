package errors

type UnverifiedEmailError struct {
	Msg string
}

func (e UnverifiedEmailError) Error() string { return e.Msg }
