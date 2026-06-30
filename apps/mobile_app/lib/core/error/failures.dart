import 'package:equatable/equatable.dart';

abstract class Failure extends Equatable {
  final String message;
  final int? statusCode;

  const Failure({required this.message, this.statusCode});

  @override
  List<Object?> get props => [message, statusCode];
}

/// Failures originating from the remote API.
class ServerFailure extends Failure {
  const ServerFailure({required super.message, super.statusCode});
}

/// Failures originating from local persistence (secure storage, DB, etc.).
class CacheFailure extends Failure {
  const CacheFailure({required super.message});
}

/// No internet connectivity.
class NetworkFailure extends Failure {
  const NetworkFailure({
    super.message = 'No internet connection. Please check your network.',
  });
}

/// The user is not authenticated (e.g. expired/missing token).
class AuthFailure extends Failure {
  const AuthFailure({required super.message, super.statusCode});
}

/// Input validation failed before hitting the network.
class ValidationFailure extends Failure {
  const ValidationFailure({required super.message});
}

/// A catch-all for truly unexpected errors.
class UnexpectedFailure extends Failure {
  const UnexpectedFailure({
    super.message = 'An unexpected error occurred. Please try again.',
  });
}
