# Fly.io Deployment Guide

Follow these steps to deploy the stability-go service on Fly.io:

1. Authenticate with Fly.io:
   ```
   fly auth login
   ```

2. Initialize the application (from your project directory):
   ```
   fly launch --name stability-go
   ```
   (This will use the existing fly.toml configuration)

3. Set your Stability API key as a secret:
   ```
   fly secrets set STABILITY_API_KEY=your_api_key_here
   ```

4. Deploy the application:
   ```
   fly deploy
   ```

5. Check your application status:
   ```
   fly status
   ```

6. View the application logs:
   ```
   fly logs
   ```

7. Open your application in the browser:
   ```
   fly open
   ```