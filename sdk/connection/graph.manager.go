package connection

//import (
//    "errors"
//
//    ultipa " github.com/ultipa/ultipa-go-driver/rpc"
//
//    "github.com/lrita/cmap"
//)
//
//type GraphManager struct {
//    graphs *cmap.Cmap
//}
//
//func NewGraphManager() *GraphManager {
//    return &GraphManager{
//        graphs: &cmap.Cmap{},
//    }
//}
//func (gm *GraphManager) DeleteGraph(graphName string) {
//    gm.graphs.Delete(graphName)
//}
//func (gm *GraphManager) AddGraph(graphName string) {
//
//    gm.graphs.LoadOrStore(graphName, &GraphClusterInfo{
//        Graph: graphName,
//    })
//
//}
//
//func (gm *GraphManager) GetGraph(graphName string) *GraphClusterInfo {
//    gci, ok := gm.graphs.Load(graphName)
//
//    if ok == false {
//        return nil
//    }
//
//    return gci.(*GraphClusterInfo)
//}
//
//func (gm *GraphManager) GetLeader(graphName string) *Connection {
//
//    gci := gm.GetGraph(graphName)
//
//    if gci == nil {
//        return nil
//    }
//
//    return gci.Leader
//}
//
//func (gm *GraphManager) SetLeader(graphName string, conn *Connection) {
//
//    _, ok := gm.graphs.Load(graphName)
//
//    if ok == false {
//        gm.AddGraph(graphName)
//    }
//
//    gci := gm.GetGraph(graphName)
//
//    if gci != nil {
//        //TODO: concurrent conflict
//        gci.Leader = conn
//    }
//}
//
//func (gm *GraphManager) ClearFollower(graphName string) {
//    gci := gm.GetGraph(graphName)
//
//    if gci == nil {
//        return
//    }
//
//    gci.Followers = []*Connection{}
//    gci.Algos = []*Connection{}
//}
//
//func (gm *GraphManager) AddFollower(graphName string, conn *Connection) {
//
//    gci := gm.GetGraph(graphName)
//
//    if gci == nil {
//        return
//    }
//
//    gci.Graph = conn.Host
//
//    if gci.HasConn(conn) == true {
//        return
//    }
//
//    gci.Followers = append(gci.Followers, conn)
//
//    if conn.HasRole(ultipa.FollowerRole_ROLE_ALGO_EXECUTABLE) {
//        gci.Algos = append(gci.Algos, conn)
//    }
//}
//
//func (gci *GraphClusterInfo) GetAnalyticConn() (*Connection, error) {
//
//    if len(gci.Algos) == 0 {
//        return nil, errors.New("no Algo/Task Instance Found")
//    }
//
//    gci.LastAlgoIndex++
//
//    return gci.Algos[gci.LastAlgoIndex%len(gci.Algos)], nil
//}
//
//func (gci *GraphClusterInfo) HasConn(_conn *Connection) bool {
//
//    if gci.Leader == _conn {
//        return true
//    }
//
//    for _, conn := range gci.Followers {
//        if conn == _conn {
//            return true
//        }
//    }
//
//    return false
//}
