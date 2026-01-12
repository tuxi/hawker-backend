## ubuntu 安装最新版pgsql

- 1.‌更新你的包列表‌：
```
sudo apt update
```

- 2.安装最新版pgsql
```
sudo apt install postgresql postgresql-contrib
```

- 3.设置密码
```
# 切换到 PostgreSQL 用户
sudo -i -u postgres
# 进入 PostgreSQL 命令行
psql
# 修改密码,在 psql 提示符下执行以下命令
ALTER USER postgres WITH PASSWORD 'your_new_password';
# 退出
\q
# 返回系统用户
exit
```

- 3.访问 数据库
```
psql -U postgres -h localhost -W
# 然后输入上面设置的密码
```
- 4.创建数据库
```postgresql
-- 如果数据库已存在，这句会报错，属于正常现象
CREATE DATABASE hawker_db;
```

- 5.检查运行状态
```
-
```

- 6.配置允许远程访问
> 修改 pg_hba.conf 文件
> 该文件用于配置用户访问权限。默认路径为 /etc/postgresql/<版本>/main/pg_hba.conf，例如 /etc/postgresql/12/main/pg_hba.conf
```
sudo vim /etc/postgresql/16/main/pg_hba.conf
```
在文件末尾添加以下内容，允许特定 IP 段访问（例如 192.168.1.0/24 网段）,如果允许所有 IP 访问，可改为 0.0.0.0/0：
```
# IPv4 remote connections
host    all             all             0.0.0.0/0            md5
```

> 修改 postgresql.conf 文件
> 该文件用于配置服务器监听模式。默认路径为 /etc/postgresql/<版本>/main/postgresql.conf，例如 /etc/postgresql/12/main/postgresql.conf
```
sudo vim /etc/postgresql/16/main/postgresql.conf
```

找到并修改以下行：
```
# 原始配置
#listen_addresses = 'localhost'

# 修改为允许所有主机访问
listen_addresses = '*'

```

- 7.重启 PostgreSQL 服务
```
sudo systemctl restart postgresql
```

- 8.配置防火墙（可选）
  确保防火墙允许 5432 端口（PostgreSQL 默认端口）：
```
sudo ufw allow 5432/tcp
```

- 9.远程连接测试
```
psql -U postgres -h <远程IP> -p 5432
```
> 提示‌：默认情况下，postgres 用户是超级用户，需先设置密码（如 ALTER USER postgres WITH PASSWORD 'your_password';）。