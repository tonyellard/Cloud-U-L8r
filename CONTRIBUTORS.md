# Contributing to Cloud-U-L8r

We welcome contributions from the community! This document outlines the process for contributing to this project.

## Code of Conduct

Please be respectful and constructive in all interactions. This project is dedicated to providing a welcoming environment for all contributors.

## How to Contribute

### Reporting Issues

- Check existing issues before creating a new one
- Clearly describe the problem and steps to reproduce
- Include relevant logs, screenshots, or error messages
- Specify your environment (OS, Go version, etc.)

### Submitting Changes

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/your-feature-name`
3. Make your changes with clear, descriptive commits
4. Add tests for new functionality
5. Ensure all tests pass: `make test`
6. Submit a pull request with a clear description of changes

### Development Setup

See individual service READMEs for detailed setup instructions:
- [ess-three](services/essthree/README.md) - S3 Emulator
- [ess-queue-ess](services/ess-queue-ess/README.md) - SQS Emulator
- [ess-enn-ess](services/ess-enn-ess/README.md) - SNS Emulator
- [cloudfauxnt](services/cloudfauxnt/README.md) - CloudFront Emulator

### Code Standards

- Write clear, readable code with meaningful variable names
- Add comments for complex logic
- Follow language-specific conventions (Go, Python, TypeScript, etc.)
- Keep functions focused and reasonably sized
- Include tests for new features

### Testing

Each service has its own test suite:

```bash
cd services/<service-name>
make test
```

Run integration tests from the root:

```bash
make integration-test
```

## License

By contributing to this project, you agree that your contributions will be licensed under the Apache License 2.0, as described in the [LICENSE](LICENSE) file.

## Questions?

Feel free to open an issue with your question or check the documentation in each service's README.

Thank you for contributing to Cloud-U-L8r!
