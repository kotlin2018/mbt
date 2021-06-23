# 用户管理表
create table if not exists `ums_admin`(
    `id`          bigint primary key auto_increment,
    `parent_id`   bigint comment '这是一个逻辑外键,关联它自身ums_admin(id)',
    `username`    varchar(64),
    `password`    varchar(64),
    `icon`        varchar(500) comment '头像',
    `email`       varchar(100) comment '邮箱',
    `nick_name`   varchar(200) comment '昵称',
    `note`        varchar(500) comment '备注信息',
    `create_time` datetime default CURRENT_TIMESTAMP comment '创建时间',
    `login_time`  datetime default CURRENT_TIMESTAMP comment '最后登录时间',
    `status`      int(1) comment '帐号启用状态：0->禁用；1->启用'
)ENGINE=InnoDB DEFAULT CHARSET=utf8 ROW_FORMAT=COMPACT COMMENT='用户管理表';;

create table if not exists `ums_role`(
    `id`          bigint primary key auto_increment,
    `name`        varchar(100) comment '名称',
    `description` varchar(500) comment '描述',
    `admin_count` int comment '后台用户数量',
    `create_time` datetime default CURRENT_TIMESTAMP comment '创建时间',
    `status`      int(1) comment '启用状态：0->禁用；1->启用',
    `sort`        Int
) ENGINE=InnoDB DEFAULT CHARSET=utf8 ROW_FORMAT=COMPACT comment '用户角色表';