# TB API Password Configuration Guide

## Security & Concepts

### 1. Security & Usability

- **Protection**: Plaintext passwords must not be exposed in public repositories (e.g., GitHub upstream), even for development. Storing them as **bcrypt hashes** eliminates this risk.
- **User Experience**: Users can use **plaintext passwords** naturally when making API calls.

### 2. Implementation Requirements

- **Secure Transmission**: Since the API receives plaintext passwords, **End-to-End Encryption (e.g., HTTPS)** is mandatory to ensure secure delivery.
- **Special Character Handling**: Bcrypt hashes contain special characters (e.g., `$`). These must be escaped differently depending on the configuration format.
  - _See the [Configuring the Password](#configuring-the-password) section for examples (e.g., `$$` in `docker-compose`)._

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
