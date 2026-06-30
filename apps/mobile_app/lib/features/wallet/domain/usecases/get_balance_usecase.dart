import 'package:fpdart/fpdart.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../entities/wallet_entity.dart';
import '../repositories/wallet_repository.dart';

class GetBalanceUseCase implements UseCase<WalletEntity, NoParams> {
  const GetBalanceUseCase(this._repository);

  final WalletRepository _repository;

  @override
  Future<Either<Failure, WalletEntity>> call(NoParams params) =>
      _repository.getBalance();
}
