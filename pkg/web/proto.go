package web

// 通用响应结构体
// 成功时 HTTP 状态码为 200，body 中 code 为 200；
// 失败时 HTTP 状态码与 body 中 code 一致（400/401/404/409/500 等），
// message 为提示信息，data 为数据内容。

// Response 统一响应结构体
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// 辅助方法
func Success(data any) Response {
	return Response{
		Code:    200,
		Message: "success",
		Data:    data,
	}
}

func Error(code int, msg string) Response {
	return Response{
		Code:    code,
		Message: msg,
		Data:    nil,
	}
}

type CreateIPNetRequest struct {
	IpNet   string `json:"ip_net"`
	GroupId uint   `json:"group_id"`
	Action  string `json:"action"`
}

type ImportIPNetRequest struct {
	Text string `json:"text"`
	Url  string `json:"url"`

	GroupId uint   `json:"group_id"`
	Action  string `json:"action"`
}

type ImportIPNetResponse struct {
	SuccessCount int `json:"success_count"`
	FailedCount  int `json:"failed_count"`
}

// BatchDeleteIPNetRequest 批量删除IP规则请求
type BatchDeleteIPNetRequest struct {
	IDs []uint `json:"ids"`
}

// BatchUpdateIPNetActionRequest 批量修改IP规则动作请求
type BatchUpdateIPNetActionRequest struct {
	IDs    []uint `json:"ids"`
	Action string `json:"action"`
}

// BatchUpdateIPNetGroupRequest 批量修改IP所属组请求
type BatchUpdateIPNetGroupRequest struct {
	IDs     []uint `json:"ids"`
	GroupId uint   `json:"group_id"`
}

// BatchOperationResponse 批量操作结果
type BatchOperationResponse struct {
	SuccessCount int `json:"success_count"`
	FailedCount  int `json:"failed_count"`
}

// UpdateIPNetGroupRequest 修改IP所属组请求
type UpdateIPNetGroupRequest struct {
	ID      uint `json:"id"`
	GroupId uint `json:"group_id"`
}

type UpdateIPNetActionRequest struct {
	ID     uint   `json:"id"`
	Action string `json:"action"`
}

// CreateGroupRequest 组管理请求
type CreateGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// UpdateGroupRequest 更新组请求
type UpdateGroupRequest struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
