### Fixed

- Web page reads (`internet.open`) no longer open addresses on your own computer or local network, such as Yggdrasil's own API, a router's admin page, or a cloud metadata service. A web page could otherwise lead the assistant there and read the result. Every address a site's name resolves to is checked, again on each connection, so a redirect or a changing name cannot get around it.
