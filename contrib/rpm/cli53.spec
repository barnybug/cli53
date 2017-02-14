%global debug_package   %{nil}
%global import_path     github.com/barnybug/cli53
%global build_path      %{_builddir}/src/%{import_path}

Name:           cli53
Version:        0.8.7
Release:        1%{?dist}
Summary:        Command line tool for Amazon Route 53
License:        MIT
URL:            https://%{import_path}
Source0:        https://%{import_path}/archive/%{version}.tar.gz
BuildRequires:	golang >= 1.5

%description
Provides import and export from BIND format and simple command line management of Route 53 domains.
Features:
    Import and export BIND format
    Create, delete and list hosted zones
    Create, delete and update individual records
    Create AWS extensions: failover, geolocation, latency, weighted and ALIAS records
    Create, delete and use reusable delegation sets

%prep
%setup -q
%{__mkdir_p} %{build_path}
%{__cp} -R ./* %{build_path}

%build
export GOPATH=%{_builddir}
cd %{build_path}
%{__make} build

%install
%{__mkdir_p} %{buildroot}/%{_bindir}
%{__install} --preserve-timestamps --mode 755 %{build_path}/%{name} %{buildroot}%{_bindir}/%{name}

%clean
%{__rm} -rf %{buildroot}

%files
%attr(0755, root, root) %{_bindir}/%{name}

%changelog
* Mon Feb 13 2017 Daniel Aharon <dan@danielaharon.com> - 0.8.7-1
- Initial
