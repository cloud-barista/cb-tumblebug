default:
	cd src/ && $(MAKE)

cc:
	cd src/ && $(MAKE) cc

run:
	cd src/ && $(MAKE) run

runwithport:
	cd src/ && $(MAKE) runwithport $(PORT)

prod:
	cd src/ && $(MAKE) prod

clean:
	cd src/ && $(MAKE) clean

swag swagger:
	cd src/ && $(MAKE) swag
