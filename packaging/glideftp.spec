Name:           glideftp
Version:        @VERSION@
Release:        1%{?dist}
Summary:        Desktop FTP/SFTP client with dual-panel interface
License:        MIT
URL:            https://github.com/Quirky1869/GlideFTP
ExclusiveArch:  x86_64

# Dependency package names vary per distro:
#   Fedora/RHEL : webkit2gtk4.1
#   openSUSE    : libwebkit2gtk-4_1-0
Requires: webkit2gtk4.1

%description
GlideFTP is a modern desktop FTP/SFTP client built with Go, Wails v2 and Svelte.
It features a dual-panel interface, transfer queue, tree view, site manager with
system keyring password storage, and support for FTP, FTPS and SFTP protocols.

%install
install -Dm755 %{_sourcedir}/GlideFTP         %{buildroot}%{_bindir}/glideftp
install -Dm644 %{_sourcedir}/glideftp.desktop %{buildroot}%{_datadir}/applications/glideftp.desktop
install -Dm644 %{_sourcedir}/glideftp.png     %{buildroot}%{_datadir}/icons/hicolor/256x256/apps/glideftp.png

%files
%{_bindir}/glideftp
%{_datadir}/applications/glideftp.desktop
%{_datadir}/icons/hicolor/256x256/apps/glideftp.png

%changelog
* @CHANGELOG_DATE@ Quirky1869 <mikec18reggae@gmail.com> - @VERSION@-1
- Release @VERSION@
