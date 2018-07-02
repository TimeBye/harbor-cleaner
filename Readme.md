# Harbor cleaner
按照`project`和`tag`名称中是否包含定义的关键字或匹配的正则表达式进行删除。此操作只删除数据库索引，不删除镜像所占存储，删除镜像所占存储请参考[官方文档](https://github.com/vmware/harbor/blob/master/docs/user_guide.md#deleting-repositories)。

**注意** `project`中若有镜像是不会进行删除的。

⚠️ **警告** ⚠️ 如果`tag A`和`tag B`都指向同一个`image`，那么当你在删除`tag A`时，`tag B`也将被删除。

# 安装

```bash
go get -u github.com/TimeBye/harbor-cleaner
```

# 使用

- 编写配置文件`delete_policy.yml`
```yaml
# 仓库相关信息
registry_url: https://registry.example.com/
username: admin
password: password

# 仅模拟运行，不真实删除，默认启用
dry_run: true
# 删除以现在时间为基础以前的镜像，单位为小时，默认72
interval_hour: 72
# 至少保留镜像个数，默认10
mix_count: 10
# 忽略这个项目下所有镜像
ignore_projects:
# 项目删除策略
projects:
  # 是否删除空项目
  delete_empty: false
  # 需删除的关键字
  include:
    # 按关键字进行删除
    keys:
    # 按正则表达式删除
    regex:
  # 排除策略，删除策略与排除策略都匹配，以排除策略为准
  exclude:
    # 按关键字进行排除
    keys:
    # 按正则表达式排除
    regex:

# 镜像tag删除策略
tags:
  # 删除策略
  include:
    # 按关键字进行删除
    keys: dev,test
    # 按正则表达式删除
    regex:
  # 排除策略，删除策略与排除策略都匹配，以排除策略为准
  exclude:
    # 按关键字进行排除
    keys:
    # 按正则表达式排除
    regex: latest|master|^[Vv]?(\d+(\.\d+){1,2})$
```

- 运行并指定配置文件位置

```bash
harbor-cleaner -f delete_policy.yml
```