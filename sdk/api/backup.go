package api

import (
	"encoding/json"
	"fmt"
	"github.com/ultipa/ultipa-go-driver/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/sdk/http"
	"github.com/ultipa/ultipa-go-driver/sdk/structs"
	"github.com/ultipa/ultipa-go-driver/sdk/utils"
)

func (api *UltipaAPI) ShowBacukup(config *configuration.RequestConfig) (backupinfo []*structs.BackupInfo, err error) {
	uql := fmt.Sprintf("db.backup.show()")

	return api.backup(uql, config)
}

func (api *UltipaAPI) backup(uql string, config *configuration.RequestConfig) (backupinfos []*structs.BackupInfo, err error) {
	resp, err := api.Uql(uql, config)

	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf(resp.Status.Message)
	}

	table, err := resp.Alias(http.RESP_BACKUP_KEY).AsTable()
	if err != nil {
		return nil, err
	}

	//printers.PrintTable(table)

	for _, values := range table.ToKV() {
		var backup []structs.ShardInfo
		status := values.Get("status").(string)
		if status == "DONE" {
			bf := values.Get("backup_infos").(string)
			err = json.Unmarshal([]byte(bf), &backup)
			if err != nil {
				return nil, err
				//api.Logger.Warn(bf + ": backup_infos Unmarshal error" + err.Error())
			}
		}

		var startTime, endTime string

		switch v := values.Get("start_time").(type) {
		case *utils.UltipaTime:
			if v != nil {
				startTime = v.String()
			}
		case string:
			startTime = v
		default:
			startTime = ""
		}

		switch v := values.Get("end_time").(type) {
		case *utils.UltipaTime:
			if v != nil {
				endTime = v.String()
			}
		case string:
			endTime = v
		default:
			endTime = ""
		}

		backupinfo := &structs.BackupInfo{
			BackupName:  values.Get("backup_name").(string),
			BackupUUID:  values.Get("backup_uuid").(string),
			StartTime:   startTime,
			EndTime:     endTime,
			Status:      values.Get("status").(string),
			Msg:         values.Get("msg").(string),
			BackupInfos: backup,
		}

		backupinfos = append(backupinfos, backupinfo)
	}

	return backupinfos, nil
}
