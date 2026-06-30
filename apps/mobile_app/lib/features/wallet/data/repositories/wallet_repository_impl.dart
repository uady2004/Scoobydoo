import 'package:fpdart/fpdart.dart';

import '../../../../core/error/exceptions.dart';
import '../../../../core/error/failures.dart';
import '../../domain/entities/wallet_entity.dart';
import '../../domain/repositories/wallet_repository.dart';
import '../datasources/wallet_remote_datasource.dart';

class WalletRepositoryImpl implements WalletRepository {
  const WalletRepositoryImpl({required WalletRemoteDataSource dataSource})
      : _dataSource = dataSource;

  final WalletRemoteDataSource _dataSource;

  @override
  Future<Either<Failure, WalletEntity>> getBalance() async {
    try {
      final model = await _dataSource.getBalance();
      return Right(model);
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode));
    } catch (e) {
      return Left(UnexpectedFailure(message: e.toString()));
    }
  }

  @override
  Future<Either<Failure, List<CoinPackageEntity>>> getCoinPackages() async {
    try {
      final models = await _dataSource.getCoinPackages();
      return Right(models);
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode));
    } catch (e) {
      return Left(UnexpectedFailure(message: e.toString()));
    }
  }

  @override
  Future<Either<Failure, List<TransactionEntity>>> getTransactions({
    String? cursor,
  }) async {
    try {
      final models = await _dataSource.getTransactions(cursor: cursor);
      return Right(models);
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode));
    } catch (e) {
      return Left(UnexpectedFailure(message: e.toString()));
    }
  }

  @override
  Future<Either<Failure, Map<String, dynamic>>> createPaymentIntent(
    String packageId,
  ) async {
    try {
      final result = await _dataSource.createPaymentIntent(packageId);
      return Right(result);
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode));
    } catch (e) {
      return Left(UnexpectedFailure(message: e.toString()));
    }
  }

  @override
  Future<Either<Failure, WalletEntity>> confirmPurchase(
    String paymentIntentId,
  ) async {
    try {
      final model = await _dataSource.confirmPurchase(paymentIntentId);
      return Right(model);
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode));
    } catch (e) {
      return Left(UnexpectedFailure(message: e.toString()));
    }
  }

  @override
  Future<Either<Failure, void>> withdraw({
    required int amount,
    required String method,
  }) async {
    try {
      await _dataSource.withdraw(amount: amount, method: method);
      return const Right(null);
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode));
    } catch (e) {
      return Left(UnexpectedFailure(message: e.toString()));
    }
  }
}
