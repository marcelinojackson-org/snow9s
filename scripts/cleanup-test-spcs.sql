-- Clean up test resources when done
DROP SERVICE IF EXISTS nginx_service;
DROP SERVICE IF EXISTS echo_service;
DROP SERVICE IF EXISTS batch_job;
DROP COMPUTE POOL IF EXISTS test_pool;
DROP IMAGE REPOSITORY IF EXISTS test_images;