package domain

import "errors"

var ErrRecordNotFound = errors.New("record not found")

var ErrRecordAlreadyExists = errors.New("record already exists")

var ErrResourceInUse = errors.New("resource is in use")

var ErrSelfDelete = errors.New("cannot delete the current user")

var ErrLastAdmin = errors.New("cannot delete the last active administrator")
