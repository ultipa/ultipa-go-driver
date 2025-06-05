package api

import (
	"github.com/ultipa/ultipa-go-driver/v5/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/models"
)

func (api *UltipaAPI) InitModel(model *models.GraphModel, config *configuration.RequestConfig) error {

	var err error

	// check if graph is exist

	graphExist, err := api.HasGraph(model.Graph.Name, config)

	if err != nil {
		return err
	}

	if graphExist == false {
		_, err = api.CreateGraph(model.Graph, nil)
		if err != nil {
			return err
		}
	}

	//err = api.SetCurrentGraph(model.Graph.Name)

	if err != nil {
		return err
	}

	for _, schema := range model.Schemas {

		rwc, err := api.CreateSchemaIfNotExist(schema, false, config)

		if err != nil {
			return err
		}

		if rwc.Exist == true {
			// if schema is existed, try ti create properties
			for _, property := range schema.Properties {

				_, err := api.CreatePropertyIfNotExist(schema.DBType, property, nil)

				if err != nil {
					return err
				}

			}

		}
	}

	return nil
}
