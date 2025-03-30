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

3. Set your secrets:
   ```
   # Set your Stability API key (required)
   fly secrets set STABILITY_API_KEY=your_api_key_here
   
   # Set a client API key for your applications to use (optional but recommended)
   fly secrets set CLIENT_API_KEY=your_client_api_key_here
   ```
   
   Note: If you don't set a CLIENT_API_KEY, one will be automatically generated and displayed in the logs on startup.

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