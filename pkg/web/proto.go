package web

// 通用响应结构体
// code 遵循http code，错误用code区分，http status一般返回200
// message为提示信息，data为数据内容

// Response 统一响应结构体
// code: 200成功，其他为错误码
// message: 提示信息
// data: 返回数据

// 通用请求结构体
// 例如IP操作请求

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
