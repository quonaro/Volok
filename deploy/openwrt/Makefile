include $(TOPDIR)/rules.mk

PKG_NAME:=volok
PKG_VERSION:=0.1.0
PKG_RELEASE:=1

PKG_BUILD_DIR:=$(BUILD_DIR)/$(PKG_NAME)-$(PKG_VERSION)

include $(INCLUDE_DIR)/package.mk

define Package/volok
  SECTION:=net
  CATEGORY:=Network
  TITLE:=Volok VLESS node library
  URL:=https://github.com/quonaro/Volok
endef

define Package/volok/description
  Volok is a minimal VLESS node library with a CLI and HTTP subscription
  server. This package expects a prebuilt binary for the target architecture.
endef

define Package/volok/conffiles
/etc/volok/volok.json
endef

define Build/Prepare
	mkdir -p $(PKG_BUILD_DIR)
	cp $(CURDIR)/volok $(PKG_BUILD_DIR)/volok
endef

define Build/Compile
	# Prebuilt binary — nothing to compile.
endef

define Package/volok/install
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) $(PKG_BUILD_DIR)/volok $(1)/usr/bin/volok

	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/volok.init $(1)/etc/init.d/volok

	$(INSTALL_DIR) $(1)/etc/volok
endef

define Package/volok/postinst
#!/bin/sh
/etc/init.d/volok enable 2>/dev/null
exit 0
endef

define Package/volok/prerm
#!/bin/sh
/etc/init.d/volok stop 2>/dev/null
/etc/init.d/volok disable 2>/dev/null
exit 0
endef

$(eval $(call BuildPackage,volok))
