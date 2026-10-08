package panel

type ApiPanel struct {
	ItemIcon      ItemIcon
	UserConfig    UserConfig
	UsersApi      UsersApi
	ItemIconGroup ItemIconGroup
	// R导航新增 (V2表结构)
	ItemCard      ItemCard
	ItemCardGroup ItemCardGroup
	// R导航新增 (Docker管理器)
	Docker Docker
}
