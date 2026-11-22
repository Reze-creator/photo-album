# API 约定

统一前缀为 `/api/v1`。除图片响应外，返回 JSON。

```json
{ "code": 0, "message": "ok", "data": {} }
```

`code` 为 `0` 表示成功。错误返回对应 HTTP 状态码和中文 `message`，`data` 为 `null`。Nginx 自身产生的网关错误可能返回 HTML，前端会显示通用错误提示。

| 方法 | 路径 | 权限与输入 |
| --- | --- | --- |
| GET | `/api/v1/health` | 公开。数据库可连接时返回 `data.status: "ok"` |
| POST | `/api/v1/register` | 公开。`multipart/form-data`，字段为 `username`、`password`、`avatar` |
| POST | `/api/v1/login` | 公开。JSON，字段为 `username`、`password`，返回 `data.token` 和 `data.user` |
| POST | `/api/v1/albums` | 登录。JSON，字段为 `name`、可选 `description`、`is_public` |
| GET | `/api/v1/albums` | 登录。返回自己的相册，支持分页 |
| GET | `/api/v1/albums/public` | 公开。分页返回公开相册及 `creator` 用户名和头像 |
| GET | `/api/v1/albums/:id` | 公开相册对访客可见，私有相册限主人 |
| GET | `/api/v1/albums/:id/photos` | 与相册详情的读取规则一致，支持分页 |
| POST | `/api/v1/albums/:id/photos` | 仅相册主人。`multipart/form-data`，文件字段为 `photo` |
| GET | `/uploads/avatars/:name` | 公开。返回头像图片 |
| GET / HEAD | `/uploads/photos/:name` | 与所属相册的读取规则一致 |

登录后把 `Authorization: Bearer <JWT>` 放入请求头。缺少有效登录时返回 `401`，已登录的其他用户访问私有相册或试图向他人相册上传时返回 `403`。

分页使用 `?page=1&page_size=12`。页码从 1 开始，每页默认 12 条、最多 50 条，结果按 ID 倒序。典型分页响应如下。

```json
{
  "code": 0,
  "message": "ok",
  "data": { "items": [], "total": 0, "page": 1, "page_size": 12 }
}
```

上传支持 JPG、PNG 和 GIF。头像最多 2 MB，照片最多 10 MB。`file_url` 与 `avatar_url` 是浏览器可请求的相对 URL。私有原图也需要 Bearer Token。
