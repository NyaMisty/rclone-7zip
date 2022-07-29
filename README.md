# rclone-7zip

Directly extract any format 7zip supports to Rclone

(Currently [NyaMisty/fclone](https://github.com/NyaMisty/fclone) is needed as we added extra `rclone rc` command)

## Features

- **Format Support**: support all format that 7-zip supports (more precisely, lib7zip supports)
  - [x] Password protected archive
  - [x] Filename encrypted archive
  - [x] Multi-Volume archive
- **Memory optimized**: transfer both large-files & small files in high speed
  - Adaptive transmit buffer, allowing maxmizing Rclone's power
- **Auto Retry**: retry uploading when rclone fails accidentally

## How to use

1. Download [NyaMisty/fclone](https://github.com/NyaMisty/fclone)'s release and run it with:
   - `rclone rcd --rc-web-gui --rc-no-auth -vvP`
2. Download release of this repo
3. Download [NyaMisty/libc7zip](https://github.com/NyaMisty/libc7zip)'s release and put it under same folder
4. Run `ln -s /usr/lib/p7zip/7z.so` in the same folder to link 7z.so here
5. Run `rclone-7zip --help` to get the usage
   - Example:
      ```
      ~/rclone-7zip/rclone-7zip --password cychd.top /tmp/tarmount_T588-21.tar/T588-21.7z.001 onedrive_backend:ciyuanchongdong-T588-喵糖映画-cychd.top
      ```

## How it works

1. Open Archive & Extract: this tool uses modified itchio/sevenzip-go to open 7zip archives
   - I added password & multi volume support to itchio's fork
2. Connect to Rclone & Transfer: using `rcatsize` operation exposed by modified `rclone rc`
   - this tool listen to a fifo & write data to it
   - rclone connect to the fifo according to rcatsize's parameter
   - transfer the data
3. Collect Result & Retry: using `rclone rc`'s `_async` mode to queue each job and manage their status
   - Retry the extract if failed

