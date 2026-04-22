package realtime

import "fmt"

// LakeTopic 给定 lakeID 返回该湖的广播 topic。
func LakeTopic(lakeID string) string {
	return fmt.Sprintf("lake:%s", lakeID)
}

// UserTopic 返回用户级通知 topic（P14-A）。
func UserTopic(userID string) string {
	return fmt.Sprintf("user:%s:notifications", userID)
}
