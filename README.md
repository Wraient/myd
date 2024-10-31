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

## Installing and Setup

### Linux
<details>
<summary>Arch Linux / Manjaro (AUR-based systems)</summary>

Using Yay

```
yay -Sy myd
```

or using Paru:

```
paru -Sy myd
```

Or, to manually clone and install:

```bash
git clone https://aur.archlinux.org/myd.git
cd myd
makepkg -si
```
</details>

<details>
<summary> Debian / Ubuntu (and derivatives) </summary>

```bash
sudo apt update
curl -Lo myd https://github.com/Wraient/myd/releases/latest/download/myd
chmod +x myd
sudo mv myd /usr/local/bin/
myd
```
</details>

<details>
<summary>Fedora Installation</summary>

```bash
sudo dnf update
curl -Lo myd https://github.com/Wraient/myd/releases/latest/download/myd
chmod +x myd
sudo mv myd /usr/local/bin/
myd
```
</details>

<details>
<summary>openSUSE Installation</summary>

```bash
sudo zypper refresh
curl -Lo myd https://github.com/Wraient/myd/releases/latest/download/myd
chmod +x myd
sudo mv myd /usr/local/bin/
myd
```
</details>

<details>
<summary>Generic Installation</summary>

```bash
curl -Lo myd https://github.com/Wraient/myd/releases/latest/download/myd
chmod +x myd
sudo mv myd /usr/local/bin/
myd
```
</details>

<details>
<summary>Uninstallation</summary>

```bash
sudo rm /usr/local/bin/myd
```

For AUR-based distributions:

```bash
yay -R myd
```
</details>

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
