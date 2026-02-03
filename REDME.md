cd hawker-backend
go mod tidy

ubuntu 安装最新版pgsql
```
sudo apt update
# 安装最新版pgsql
sudo apt install postgresql postgresql-contrib
# 检查运行状态
sudo systemctl status postgresql
```

1. 创建数据库 
如果你在命令行（psql）或图形化界面，请先执行： 
进入数据库
```
psql -U postgres -h localhost -W
```

```postgresql
-- 如果数据库已存在，这句会报错，属于正常现象
CREATE DATABASE hawker_db;
```
2. 切换并初始化（关键步骤）
这一步非常重要：你必须先“进入”这个新创建的 hawker_db 数据库，然后再安装扩展和创建表。
如果你使用的是命令行，输入：
```Bash
\c hawker_db
```

如果你使用的是 DBeaver / Navicat：
在左侧连接列表中找到 hawker_db。
双击它确保它变成活动状态（通常颜色会变深）。
在针对该数据库打开一个新的“查询控制台（Query Console）”。

3. 运行初始化脚本`script.sql` 
一旦你确认当前连接的是 hawker_db，请运行以下完整的初始化 SQL：
如果使用gorm的db.AutoMigrate自动迁移，则不需要手动维护script.sql文件


4.安装`edge-tts`语言合成
mac 
```bash
sudo pip3 install 
```
linux
```shell
sudo apt update
sudo apt install python3-pip
pip3 install edge-tts
```

docker 运行项目
```
docker run -p 12188:12188 -v /data/hawker/conf:/app/hawker-backend/conf hawker-app
```

docker compose首次启动/代码更新后启动
```
docker-compose up -d --build
```
查看实时日志
```
docker-compose logs -f hawker-app
```
停止并移除
```
docker compose down
```