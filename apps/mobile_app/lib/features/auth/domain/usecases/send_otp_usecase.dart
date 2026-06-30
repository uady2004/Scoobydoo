import 'package:equatable/equatable.dart';
import 'package:fpdart/fpdart.dart';
import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../repositories/auth_repository.dart';

class SendOtpUseCase implements UseCase<Unit, SendOtpParams> {
  final AuthRepository _repository;

  const SendOtpUseCase(this._repository);

  @override
  Future<Either<Failure, Unit>> call(SendOtpParams params) {
    return _repository.sendOTP(phone: params.phone);
  }
}

class SendOtpParams extends Equatable {
  final String phone;

  const SendOtpParams({required this.phone});

  @override
  List<Object?> get props => [phone];
}
