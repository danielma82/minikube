## Requirements

- libvirt v1.3.1 or higher
- qemu-kvm v2.0 or higher

## Installing Prerequisites

Proper installation of KVM and libvirt is highly specific to each Linux distribution. Please consult:

* [ArchLinux](https://wiki.archlinux.org/index.php/Libvirt)
* [Debian](https://wiki.debian.org/KVM#Installation)
* [Fedora](https://docs.fedoraproject.org/en-US/quick-docs/getting-started-with-virtualization/)
* [Gentoo](https://wiki.gentoo.org/wiki/QEMU)
* [OpenSUSE](https://doc.opensuse.org/documentation/leap/virtualization/html/book.virt/cha.vt.installation.html)
* [RedHat](https://access.redhat.com/articles/1344173#Q_how-install-virtualization-packages)
* [Ubuntu](https://help.ubuntu.com/community/KVM/Installation)

Once configured, validate that libvirt reports no errors:

```shell
virt-host-validate
```

## Driver installation

```shell
 curl -LO https://storage.googleapis.com/minikube/releases/latest/docker-machine-driver-kvm2 \
  && sudo install docker-machine-driver-kvm2 /usr/local/bin/
```
