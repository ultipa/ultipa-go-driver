package structs

import "github.com/ultipa/ultipa-go-driver/sdk/types"

type MetaData struct {
	ID     types.ID
	UUID   types.UUID
	From   types.ID
	To     types.ID
	Schema string
	Values *Values
}
