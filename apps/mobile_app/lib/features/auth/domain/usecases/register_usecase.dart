import 'package:equatable/equatable.dart';
import 'package:fpdart/fpdart.dart';
import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../entities/auth_token.dart';
import '../repositories/auth_repository.dart';

class RegisterUseCase implements UseCase<AuthToken, RegisterParams> {
  final AuthRepository _repository;

  const RegisterUseCase(this._repository);

  @override
  Future<Either<Failure, AuthToken>> call(RegisterParams params) {
    return _repository.register(
      username: params.username,
      email: params.email,
      password: params.password,
      phone: params.phone,
    );
  }
}

class RegisterParams extends Equatable {
  final String username;
  final String email;
  final String password;
  final String? phone;

  const RegisterParams({
    required this.username,
    required this.email,
    required this.password,
    this.phone,
  });

  @override
  List<Object?> get props => [username, email, password, phone];
}
