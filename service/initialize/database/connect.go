package database

import (
	"log"
	"os"
	"path"
	"sun-panel/lib/cmn"
	"sun-panel/models"
	"time"

	"gorm.io/driver/mysql"
	_ "gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	_ "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

const (
	MYSQL  = "mysql"
	SQLITE = "sqlite"
)

type DbClient interface {
	Connect() (db *gorm.DB, err error)
}

type MySQLConfig struct {
	Username    string
	Password    string
	Host        string
	Port        string
	Database    string
	WaitTimeout int
}

type SQLiteConfig struct {
	Filename string
}

func DbInit(dbClient DbClient) (db *gorm.DB, dbErr error) {
	db, dbErr = dbClient.Connect()
	if dbErr != nil {
		return
	}
	return
}

// Connect mysql连接
func (d *MySQLConfig) Connect() (db *gorm.DB, err error) {
	dsn := d.Username + ":" + d.Password + "@tcp(" + d.Host + ":" + d.Port + ")/" + d.Database + "?charset=utf8mb4&parseTime=True&loc=Local"
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: GetLogger(),
		NamingStrategy: schema.NamingStrategy{
			// TablePrefix:   "blog_",
			SingularTable: true,
		},
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	sqlDb, _ := db.DB()
	sqlDb.SetMaxIdleConns(10)  // SetMaxIdleConns 设置空闲连接池中连接的最大数量
	sqlDb.SetMaxOpenConns(100) // SetMaxOpenConns 设置打开数据库连接的最大数量。
	wait_timeout := d.WaitTimeout
	sqlDb.SetConnMaxLifetime(time.Duration(wait_timeout * int(time.Second))) // SetConnMaxLifetime 设置了连接可复用的最大时间。
	return
}

// Connect sqllite3连接
func (d *SQLiteConfig) Connect() (db *gorm.DB, err error) {
	filePath := d.Filename
	exists := false
	if exists, err = cmn.PathExists(path.Dir(filePath)); err != nil {
		return
	} else {

		// 创建文件夹
		if !exists {
			if err = os.MkdirAll(path.Dir(filePath), 0700); err != nil {
				return
			}
		}

		// OPT-012：SQLite 并发与可靠性优化
		// OPT-014：pragma 移入 DSN —— 连接池每个新连接自动应用；
		// 之前 db.Exec 只对建库时的首连接有效，池中新连接 busy_timeout=0（已实测确认）
		dsn := filePath + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
		db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
			Logger: GetLogger(),
			NamingStrategy: schema.NamingStrategy{
				SingularTable: true,
			},
		})
		if err != nil {
			return
		}
		// WAL 模式：读不阻塞写，多读并发；busy_timeout：锁冲突时等待而非直接报错；
		// synchronous=NORMAL：WAL 下安全且更快；限制连接池避免写串行化导致的堆积
		if sqlDb, err := db.DB(); err == nil {
			sqlDb.SetMaxOpenConns(10)
			sqlDb.SetMaxIdleConns(5)
		}
	}

	return
}

// 日志
func GetLogger() logger.Interface {
	return logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer（日志输出的目标，前缀和日志包含的内容——译者注）
		logger.Config{
			SlowThreshold:             time.Second, // 慢 SQL 阈值
			LogLevel:                  logger.Warn, // 日志级别
			IgnoreRecordNotFoundError: true,        // 忽略ErrRecordNotFound（记录未找到）错误
			Colorful:                  true,        // 彩色打印
		},
	)

}

// 创建数据库
func CreateDatabase(driver string, db *gorm.DB) error {

	// mysql特殊处理
	if driver == MYSQL {
		db = db.Set("gorm:table_options", "ENGINE=InnoDB")
	}

	// 创建数据表
	err := db.AutoMigrate(
		&models.User{},
		&models.SystemSetting{},
		&models.ItemIcon{},
		&models.UserConfig{},
		&models.File{},
		&models.ItemIconGroup{},
		&models.ModuleConfig{},
		// R导航: V2表结构
		&models.ItemCard{},
		&models.ItemCardGroup{},
	)
	if err != nil {
		return err
	}

	// R导航优化：确保高频查询索引存在
	models.EnsureItemCardIndexes(db)
	models.EnsureItemCardGroupIndexes(db)
	models.EnsureUserIndexes(db) // OPT-012：rpass_id 查询索引

	return nil
}

// 初始化一个用户,一个用户都没有的时候创建一个
func NotFoundAndCreateUser(db *gorm.DB) error {
	fUser := models.User{}
	if err := db.First(&fUser).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return err
		}
		username := "admin@sun.cc"
		fUser.Mail = username
		fUser.Username = username
		fUser.Name = username
		fUser.Status = 1
		fUser.Role = 1
		fUser.Password = cmn.PasswordEncryption("12345678")

		if errCreate := db.Create(&fUser).Error; errCreate != nil {
			return errCreate
		}
	}

	return nil
}
