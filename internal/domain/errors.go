package domain

import "errors"

var ErrRecordNotFound = errors.New("record not found")

var ErrRecordAlreadyExists = errors.New("record already exists")

var ErrResourceInUse = errors.New("resource is in use")
