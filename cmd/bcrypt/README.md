# TB API Password Configuration Guide

## Generating Password Hash

1. From the CB-Tumblebug root directory, generate a bcrypt hash of your password using:

   ```shell
   make bcrypt PASSWORD=yourpassword
   ```

2. Copy the generated hash value.

## Configuring the Password

### Using Docker Compose

1. Open `docker-compose.yaml` and update the `TB_API_PASSWORD` environment variable with ($$):
   ```yaml
   environment:
     - TB_API_PASSWORD=$$2a$$10$$4PKzCuJ6fPYsbCF.HR//ieLjaCzBAdwORchx62F2JRXQsuR3d9T0q
   ```

### Using Environment File

1. If you're using `setup.env`, update the password hash:
   ```shell
   TB_API_PASSWORD='$2a$10$4PKzCuJ6fPYsbCF.HR//ieLjaCzBAdwORchx62F2JRXQsuR3d9T0q'
   ```

### Using Dockerfile

1. If you're building directly with Dockerfile, update the environment variable with (' '):
   ```dockerfile
   ENV TB_API_PASSWORD='$2a$10$4PKzCuJ6fPYsbCF.HR//ieLjaCzBAdwORchx62F2JRXQsuR3d9T0q'
   ```

## Notes

- Always keep your password hash secure
- Never commit the actual password or hash to version control
- The hash should be properly escaped if it contains special characters
