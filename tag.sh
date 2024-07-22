#!/bin/bash

function checkIsOk() {
    # $1 操作名
    # $2 自定义 code, 0-成功 1-失败

    local errCode=$?
    if [[ -n $2 ]]; then
        errCode=$2
    fi

    if [[ $errCode > 0 ]]; then
        echo -e "\"${1}\" is \033[1;31mFailed\033[0m"
        exit 1
    fi
    echo -e "\"${1}\" is \033[1;32mSuccess\033[0m"
}

git pull origin master
checkIsOk "git pull"

# 获取当前分支最新的 tag
lastTag=$(git describe --abbrev=0 --tags)
checkIsOk "get last tag"
echo "最新的tag: ${lastTag}"

printf "请输入新版tag:"
read newTag
if [[ $newTag == "" ]]; then
    echo "tag 必填"
    exit 1
fi

printf "请输入版本描述:"
read versionMsg
if [[ $versionMsg == "" ]]; then
    echo "版本描述不能为空"
    exit 1
fi

# 先提交本地未提交的
git add .

git commit -am "打包 ${versionMsg}"
git push

# 打 tag
git tag -a "$newTag" -m "$versionMsg"
checkIsOk "git tag"

# 推送
git push origin $newTag
checkIsOk "git push"
