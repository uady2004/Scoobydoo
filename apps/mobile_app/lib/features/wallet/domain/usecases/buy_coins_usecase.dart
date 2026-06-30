import 'package:equatable/equatable.dart';
import 'package:fpdart/fpdart.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../entities/wallet_entity.dart';
import '../repositories/wallet_repository.dart';

// ---------------------------------------------------------------------------
// CreatePaymentIntentUseCase
// ---------------------------------------------------------------------------

class CreatePaymentIntentParams extends Equatable {
  final String packageId;
  const CreatePaymentIntentParams({required this.packageId});

  @override
  List<Object?> get props => [packageId];
}

class CreatePaymentIntentUseCase
    implements UseCase<Map<String, dynamic>, CreatePaymentIntentParams> {
  const CreatePaymentIntentUseCase(this._repository);

  final WalletRepository _repository;

  @override
  Future<Either<Failure, Map<String, dynamic>>> call(
    CreatePaymentIntentParams params,
  ) =>
      _repository.createPaymentIntent(params.packageId);
}

// ---------------------------------------------------------------------------
// ConfirmPurchaseUseCase
// ---------------------------------------------------------------------------

class ConfirmPurchaseParams extends Equatable {
  final String paymentIntentId;
  const ConfirmPurchaseParams({required this.paymentIntentId});

  @override
  List<Object?> get props => [paymentIntentId];
}

class ConfirmPurchaseUseCase
    implements UseCase<WalletEntity, ConfirmPurchaseParams> {
  const ConfirmPurchaseUseCase(this._repository);

  final WalletRepository _repository;

  @override
  Future<Either<Failure, WalletEntity>> call(ConfirmPurchaseParams params) =>
      _repository.confirmPurchase(params.paymentIntentId);
}

// ---------------------------------------------------------------------------
// WithdrawUseCase
// ---------------------------------------------------------------------------

class WithdrawParams extends Equatable {
  final int amount;
  final String method;
  const WithdrawParams({required this.amount, required this.method});

  @override
  List<Object?> get props => [amount, method];
}

class WithdrawUseCase implements UseCase<void, WithdrawParams> {
  const WithdrawUseCase(this._repository);

  final WalletRepository _repository;

  @override
  Future<Either<Failure, void>> call(WithdrawParams params) =>
      _repository.withdraw(amount: params.amount, method: params.method);
}
