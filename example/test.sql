CREATE TABLE `biz_activity`(
    `id`            varchar(50)  NOT NULL DEFAULT '' COMMENT '唯一活动码',
    `name`          varchar(255) NOT NULL default '',
    `pc_link`       varchar(255)          DEFAULT NULL,
    `h5_link`       varchar(255)          DEFAULT NULL,
    `sort`          int(11)      NOT NULL default 0 COMMENT '排序',
    `status`        int(11)      NOT NULL default 0 COMMENT '状态（0：已下线，1：已上线）',
    `version`       int(11)      NOT NULL default 0,
    `remark`        varchar(255)          DEFAULT NULL,
    `create_time`   datetime(0)           default CURRENT_TIMESTAMP,
    `delete_flag`   int(1)       NOT NULL default 0,
    `pc_banner_img` varchar(255)          DEFAULT NULL,
    `h5_banner_img` varchar(255)          DEFAULT NULL,
    PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8 ROW_FORMAT=COMPACT COMMENT='运营管理-活动管理';