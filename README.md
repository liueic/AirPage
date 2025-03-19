# AirPage

这是一个基于 TailCSS 和 Gin 的简单天气空气质量展示页面，你可以通过访问：https://air.juniortree.com 进行在线展示

![](https://s2.loli.net/2025/03/19/s2dxTWG9hZoeE4g.png)

## 自行部署

你需要获取来自 AQICN 的 API：https://aqicn.org/api/

并且将其设置到环境变量中

### 1.本地编译

```bash
git clone https://github.com/liueic/AirPage.git
```

安装相应的依赖，并且进行编辑

```bash
go mod download
go build -o airpage .
```

### 2.使用 docker

```yml
services:
  web:
    image: aicnal/airpage:latest
    ports:
      - "8080:8080"
    environment:
      - API=
    restart: unless-stopped
```