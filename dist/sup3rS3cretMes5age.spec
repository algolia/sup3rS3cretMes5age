Name: sup3rS3cretMes5age
Version: 0.3.0
Release: 1%{?dist}
Summary: A simple, secure self-destructing message service.
Group: Productivity/Networking/Security
License: MPL-2.0
URL: https://github.com/algolia/sup3rS3cretMes5age
Source0: %{name}-%{version}.tar.xz
Source1: vendor.tar.gz
Source99: %{name}.rpmlintrc

BuildRequires: golang-1.22
BuildRequires: make

BuildRoot: %{_tmppath}/%{name}-%{version}-build

%description
A simple, secure self-destructing message service, using HashiCorp Vault product as a backend.

%prep
%setup -q -n %{name}-%{version}
tar xf ../../SOURCES/vendor.tar.gz

%build
CGO_ENABLED=0 go build -buildmode=pie -mod=vendor -a -ldflags '-extldflags "-static"' -o sup3rS3cretMes5age .

%install
install -d %{buildroot}/%{_bindir}
install -m 0755 sup3rS3cretMes5age %{buildroot}/%{_bindir}

%files
%attr(0755,root,root) %{_bindir}/sup3rS3cretMes5age

%changelog
* Tue May 07 2024 Jeremy Jacque <jeremy.jacque@algolia.com> 0.3.0-1
- first package build
