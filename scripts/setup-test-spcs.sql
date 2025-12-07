-- Run this in your Snowflake account to create test SPCS resources

-- Create compute pool
CREATE COMPUTE POOL IF NOT EXISTS test_pool
  MIN_NODES = 1
  MAX_NODES = 3
  INSTANCE_FAMILY = CPU_X64_XS
  AUTO_RESUME = TRUE
  AUTO_SUSPEND_SECS = 300;

-- Create image repository
CREATE IMAGE REPOSITORY IF NOT EXISTS test_images;

-- Show repository URL (you'll need this to push images)
SHOW IMAGE REPOSITORIES LIKE 'test_images';

-- Create a simple service (nginx example)
CREATE SERVICE IF NOT EXISTS nginx_service
  IN COMPUTE POOL test_pool
  FROM SPECIFICATION $$
spec:
  containers:
  - name: nginx
    image: /test_images/nginx:latest
    env:
      SERVER_PORT: 8080
  endpoints:
  - name: http
    port: 8080
    public: true
$$;

-- Create another service (echo server)
CREATE SERVICE IF NOT EXISTS echo_service
  IN COMPUTE POOL test_pool
  FROM SPECIFICATION $$
spec:
  containers:
  - name: echo
    image: /test_images/echo:latest
    env:
      ENVIRONMENT: test
  endpoints:
  - name: api
    port: 3000
$$;

-- Create a job
CREATE SERVICE IF NOT EXISTS batch_job
  IN COMPUTE POOL test_pool
  FROM SPECIFICATION $$
spec:
  containers:
  - name: processor
    image: /test_images/processor:latest
    args: ["--mode", "batch"]
$$
MIN_INSTANCES = 1
MAX_INSTANCES = 1;

-- Grant necessary privileges (adjust role as needed)
GRANT USAGE ON COMPUTE POOL test_pool TO ROLE ACCOUNTADMIN;
GRANT MONITOR ON COMPUTE POOL test_pool TO ROLE ACCOUNTADMIN;