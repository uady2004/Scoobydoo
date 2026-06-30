import 'package:fpdart/fpdart.dart';

import '../../../../core/error/failures.dart';
import '../entities/wallet_entity.dart';

abstract class WalletRepository {
  Future<Either<Failure, WalletEntity>> getBalance();

  Future<Either<Failure, List<CoinPackageEntity>>> getCoinPackages();

  Future<Either<Failure, List<TransactionEntity>>> getTransactions({
    String? cursor,
  });

  /// Creates a Stripe PaymentIntent for the given package.
  /// Returns {clientSecret, amount}.
  Future<Either<Failure, Map<String, dynamic>>> createPaymentIntent(
    String packageId,
  );

  /// Confirms a successful payment and credits coins to the wallet.
  Future<Either<Failure, WalletEntity>> confirmPurchase(
    String paymentIntentId,
  );

  /// Withdraws diamonds as real currency via the given method.
  Future<Either<Failure, void>> withdraw({
    required int amount,
    required String method,
  });
}
