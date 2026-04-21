package realtime

import "fmt"

// LakeTopic 给定 lakeID 返回该湖的广播 topic。
func LakeTopic(lakeID string) string {
	return fmt.Sprintf("lake:%s", lakeID)
}
