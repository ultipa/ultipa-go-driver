/**
 * @Author: zhaohaichao
 * @Description:
 * @File:  index
 * @Date: 2022/8/4 3:36 pm
 */

package structs

import ultipa "github.com/ultipa/ultipa-go-driver/rpc"

type Index struct {
	Id         string
	Name       string
	Properties string
	Schema     string
	Status     string
	//Size       int
	DBType ultipa.DBType
}
