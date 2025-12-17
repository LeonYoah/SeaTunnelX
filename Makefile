swagger:
	scripts/swagger.sh

tidy:
	scripts/tidy.sh

check_license:
	scripts/license.sh

proto:
	scripts/proto.sh

pre_commit: tidy swagger check_license
