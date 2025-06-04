package printers

import (
	"github.com/alexeyco/simpletable"
	"github.com/ultipa/ultipa-go-driver/sdk/structs"
)

func PrintBackupInfoList(backupInfos []*structs.BackupInfo) {
	table := simpletable.New()

	table.Header.Cells = []*simpletable.Cell{
		//  BackupName  string      `json:"backup_name"`
		//    BackupUUID  string      `json:"backup_uuid"`
		//    StartTime   string      `json:"start_time"`
		//    EndTime     string      `json:"end_time"`
		//    Status      string      `json:"status"`
		//    Msg         string      `json:"msg"`
		//
		{Text: "backup_name"},
		{Text: "backup_uuid"},
		{Text: "start_time"},
		{Text: "end_time"},
		{Text: "status"},
		{Text: "msg"},
		{Text: "backup_infos"},
	}

	for _, backupInfo := range backupInfos {

		table.Body.Cells = append(table.Body.Cells, []*simpletable.Cell{
			{
				Text: backupInfo.BackupName,
			},
			{
				Text: backupInfo.BackupUUID,
			},
			{
				Text: backupInfo.StartTime,
			},
			{
				Text: backupInfo.EndTime,
			},
			{
				Text: backupInfo.Status,
			},
			{
				Text: backupInfo.Msg,
			},
			{
				Text: "...",
				//Text: backupInfo.BackupInfos,
			},
		})
	}

	table.Println()
}
