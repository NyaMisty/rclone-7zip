module github.com/NyaMisty/rclone-7zip

go 1.17

require (
	github.com/alecthomas/kong v0.6.1
	github.com/go-resty/resty/v2 v2.7.0
	github.com/itchio/sevenzip-go v0.0.0-20190703112252-e327cec6c376
	github.com/pkg/errors v0.8.1
	github.com/sirupsen/logrus v1.9.0
	github.com/stretchr/testify v1.7.2
	golang.org/x/sync v0.0.0-20220722155255-886fb9371eb4
	golang.org/x/sys v0.0.0-20220715151400-c0bba94af5f8
)

replace github.com/itchio/sevenzip-go v0.0.0-20190703112252-e327cec6c376 => github.com/NyaMisty/sevenzip-go v0.0.0-20220728170000-53ac00757a19
