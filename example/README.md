# example

[mbt的使用文档](https://github.com/kotlin2018/mbt.git)

* 配置文件为:
````toml
[database]  #适用于被 viper解析的配置项
    pkg = "./xml"
    driverName = "mysql"
    dsn = "root:root@(127.0.0.1:3306)/test?charset=utf8&parseTime=True&loc=Local"
    maxOpenConn = 5 #默认为5
    maxIdleConn = 10 #默认为10
    connMaxLifetime = 5 #默认5分钟
    connMaxIdleTime = 5 #默认5分钟
[database.logger]
    printSql = true
    printXml = false
    path     = "./log" #""
    linkName = "./rotate.log"
    interval = 2
    maxAge   = 180
    count    = -1 #(-1,表示不使用此配置)

[database2] #适用于被 viper解析的配置项
    pkg = "./xml"
    driverName = "mysql"
    dsn = "root:root@(127.0.0.1:3306)/test1?charset=utf8&parseTime=True&loc=Local"
    maxOpenConn = 5 #默认为5
    maxIdleConn = 10 #默认为10
    connMaxLifetime = 5 #默认5分钟
    connMaxIdleTime = 5 #默认5分钟
[database2.logger]
    printSql = true
    printXml = false
    path     = "./2log" #""
    linkName = "./2rotate.log"
    interval = 2
    maxAge   = 180
    count    = -1 #(-1,表示不使用此配置)

#[database]#适用于被yaml,toml解析的配置项
#    pkg = "./"
#    driver_name = "mysql"
#    dsn = "root:root@(127.0.0.1:3306)/test?charset=utf8&parseTime=True&loc=Local"
#    max_open_conn = 5 #默认为5
#    max_idle_conn = 10 #默认为10
#    conn_max_life_time = 5 #默认5分钟
#    conn_max_idle_time = 5 #默认5分钟
#[database.logger]
#    print_sql = true    #设置是否打印SQL语句
#    print_xml = false   #是否打印 xml文件信息
#    path = "./2log"     #日志输出路径
#    link_name = "./2rotate.log" #为最新的日志建立软连接
#    interval = 10       #设置日志分割的时间，隔多久分割一次
#    max_age = 15        #日志文件被清理前的最长保存时间
#    count = -1          #日志文件被清理前最多保存的个数
````
