package core

import "prismflow/internal/model"

func SubscriptionName(e model.Endpoint, u model.User) string { return u.Name + " · " + e.Name }
