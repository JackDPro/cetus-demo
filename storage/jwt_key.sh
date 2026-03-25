#!/bin/sh

OS_TYPE=$(uname -s)
case $OS_TYPE in
  Linux*)   OS_NAME=linux ;;
  Darwin*)  OS_NAME=macos ;;
  CYGWIN*)  OS_NAME=cygwin ;;
  MINGW*)   OS_NAME=mingw ;;
  FreeBSD*) OS_NAME=freebsd ;;
  *)        OS_NAME="unknown" ;;
esac

# 生成私钥 PKCS#1 传统 RSA 格式
openssl genrsa -out jwt1.pem 2048
# 转换成 PKCS#8 通用封装格式，并且使用 DER 封装
openssl pkcs8 -topk8 -inform PEM -outform DER -in jwt1.pem -out jwt8-der.key -nocrypt
# 转换成 PKCS#8 通用封装格式，并且使用 PEM 封装
openssl pkcs8 -topk8 -inform PEM -outform PEM -in jwt1.pem -out jwt8-pem.key -nocrypt
# 生成公钥
openssl rsa -in jwt1.pem -pubout -out jwt.pub


if [ "$OS_NAME" = "linux" ]; then
  base64 -w 0 jwt.pub > jwt-b64.pub
  base64 -w 0 jwt1.pem > jwt1-b64.pem
  base64 -w 0 jwt8-der.key > jwt8-der-b64.key
  base64 -w 0 jwt8-pem.key > jwt8-pem-b64.key
fi

if [ "$OS_NAME" = "macos" ]; then
  base64 -i jwt.pub | tr -d '\n' > jwt-b64.pub
  base64 -i jwt1.pem | tr -d '\n' > jwt1-b64.pem
  base64 -i jwt8-der.key | tr -d '\n' > jwt8-der-b64.key
  base64 -i jwt8-pem.key | tr -d '\n' > jwt8-pem-b64.key
fi
