using Amazon;
using Amazon.Runtime;
using Amazon.SecretsManager;
using Amazon.SecretsManager.Model;
using Amazon.SimpleSystemsManagement;
using Amazon.SimpleSystemsManagement.Model;

var endpointUrl = Environment.GetEnvironmentVariable("KAY_VEE_ENDPOINT_URL") ?? "http://localhost:9350";
var regionName = Environment.GetEnvironmentVariable("AWS_REGION") ?? "us-east-1";
var accessKeyId = Environment.GetEnvironmentVariable("AWS_ACCESS_KEY_ID") ?? "test";
var secretAccessKey = Environment.GetEnvironmentVariable("AWS_SECRET_ACCESS_KEY") ?? "test";

var credentials = new BasicAWSCredentials(accessKeyId, secretAccessKey);
var region = RegionEndpoint.GetBySystemName(regionName);

var ssmConfig = new AmazonSimpleSystemsManagementConfig
{
	ServiceURL = endpointUrl,
	AuthenticationRegion = regionName,
	UseHttp = true,
};

var secretsConfig = new AmazonSecretsManagerConfig
{
	ServiceURL = endpointUrl,
	AuthenticationRegion = regionName,
	UseHttp = true,
};

var suffix = DateTimeOffset.UtcNow.ToUnixTimeSeconds().ToString();
var parameterName = $"/smoke/param/dotnet/{suffix}";
var secretName = $"smoke/secret/dotnet/{suffix}";

Console.WriteLine($"[dotnet] endpoint: {endpointUrl}");
Console.WriteLine($"[dotnet] parameter: {parameterName}");
Console.WriteLine($"[dotnet] secret: {secretName}");

try
{
	using var ssmClient = new AmazonSimpleSystemsManagementClient(credentials, ssmConfig);
	using var secretsClient = new AmazonSecretsManagerClient(credentials, secretsConfig);

	await ssmClient.PutParameterAsync(new PutParameterRequest
	{
		Name = parameterName,
		Type = ParameterType.String,
		Value = "initial-value",
		Overwrite = false,
	});

	await ssmClient.PutParameterAsync(new PutParameterRequest
	{
		Name = parameterName,
		Type = ParameterType.String,
		Value = "updated-value",
		Overwrite = true,
	});

	var getParameterResponse = await ssmClient.GetParameterAsync(new GetParameterRequest
	{
		Name = parameterName,
		WithDecryption = false,
	});

	if (getParameterResponse.Parameter?.Value != "updated-value")
	{
		Console.Error.WriteLine($"[dotnet] parameter assertion failed: expected updated-value got {getParameterResponse.Parameter?.Value}");
		return 1;
	}

	await secretsClient.CreateSecretAsync(new CreateSecretRequest
	{
		Name = secretName,
		SecretString = "initial-secret",
	});

	await secretsClient.PutSecretValueAsync(new PutSecretValueRequest
	{
		SecretId = secretName,
		SecretString = "updated-secret",
	});

	var getSecretResponse = await secretsClient.GetSecretValueAsync(new GetSecretValueRequest
	{
		SecretId = secretName,
	});

	if (getSecretResponse.SecretString != "updated-secret")
	{
		Console.Error.WriteLine($"[dotnet] secret assertion failed: expected updated-secret got {getSecretResponse.SecretString}");
		return 1;
	}

	Console.WriteLine("[dotnet] smoke test passed");
	return 0;
}
catch (Exception ex)
{
	Console.Error.WriteLine($"[dotnet] smoke test failed: {ex.Message}");
	return 1;
}
