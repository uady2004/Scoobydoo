import 'package:equatable/equatable.dart';
import 'package:fpdart/fpdart.dart';
import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../entities/auth_token.dart';
import '../repositories/auth_repository.dart';

class VerifyOtpUseCase implements UseCase<AuthToken, VerifyOtpParams> {
  final AuthRepository _repository;

  const VerifyOtpUseCase(this._repository);

  @override
  Future<Either<Failure, AuthToken>> call(VerifyOtpParams params) {
    return _repository.verifyOTP(phone: params.phone, code: params.code);
  }
}

class VerifyOtpParams extends Equatable {
  final String phone;
  final String code;

  const VerifyOtpParams({required this.phone, required this.code});

  @override
  List<Object?> get props => [phone, code];
}
