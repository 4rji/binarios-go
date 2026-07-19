
Dependencies

Make sure the following dependencies are installed:

    Go >= 1.22
    git
    make


Build

Clone the project source code.

git clone https://github.com/xjasonlyu/tun2socks.git

Build and install the tun2socks binary.

make tun2socks
sudo cp ./build/tun2socks /usr/local/bin



Or build for all architectures.

make all-arch





Install

Simply use go install

go install github.com/xjasonlyu/tun2socks/v2@latest
