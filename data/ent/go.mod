module github.com/DarkInno/gotenancy/data/ent

go 1.23.0

replace github.com/DarkInno/gotenancy => ../..

require (
	entgo.io/ent v0.14.1
	github.com/DarkInno/gotenancy v0.0.0
)

require github.com/google/uuid v1.3.0 // indirect
