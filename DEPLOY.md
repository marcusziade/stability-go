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
   
   # Set allowed IP addresses for additional security (optional)
   fly secrets set ALLOWED_IPS=1.2.3.4,5.6.7.8
   
   # Set allowed app IDs for additional security (optional)
   fly secrets set ALLOWED_APP_IDS=ios-app-1,android-app-2
   ```
   
   Note: If you don't set a CLIENT_API_KEY, one will be automatically generated and displayed in the logs on startup. For maximum security, configure both ALLOWED_IPS and ALLOWED_APP_IDS.

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