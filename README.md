# MYD
Manage your dotfiles

this program help you upload and keep your github dotfiles upto date and help install them in your system

# Workflow
```
myd init
```
1. Get your github token
2. Enter your github token in the program
3. enter the repo you want in your github profile as your dots backup
4. Done

# Usage
```
myd add {PATH TO DIRECTORY OR FILE}
```
This will track the file and upload it to github


```
myd ignore {PATH TO DIRECTORY OR FILE}
```
This will ignore the files and wont upload it to github


```
myd delete
```
This will start a interactive select menu to delete added paths


```
myd upload
```
This will upload all the paths to your github


```
myd install {Github link}
```
This will install the dots at their original places (if it was uplaoded using myd.
