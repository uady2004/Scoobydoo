import 'package:equatable/equatable.dart';

// ---------------------------------------------------------------------------
// TransactionType enum
// ---------------------------------------------------------------------------

enum TransactionType { buy, gift, tip, earn, withdraw }

// ---------------------------------------------------------------------------
// WalletEntity
// ---------------------------------------------------------------------------

class WalletEntity extends Equatable {
  final int coinBalance;
  final int diamondBalance;
  final DateTime updatedAt;

  const WalletEntity({
    required this.coinBalance,
    required this.diamondBalance,
    required this.updatedAt,
  });

  @override
  List<Object?> get props => [coinBalance, diamondBalance, updatedAt];
}

// ---------------------------------------------------------------------------
// CoinPackageEntity
// ---------------------------------------------------------------------------

class CoinPackageEntity extends Equatable {
  final String id;
  final int coins;
  final int bonusCoins;
  final double price;
  final String currency;
  final bool isBestValue;

  const CoinPackageEntity({
    required this.id,
    required this.coins,
    required this.bonusCoins,
    required this.price,
    required this.currency,
    required this.isBestValue,
  });

  int get totalCoins => coins + bonusCoins;

  @override
  List<Object?> get props => [id, coins, bonusCoins, price, currency, isBestValue];
}

// ---------------------------------------------------------------------------
// TransactionEntity
// ---------------------------------------------------------------------------

class TransactionEntity extends Equatable {
  final String id;
  final TransactionType type;
  final int amount;
  final String currency;
  final String referenceId;
  final String description;
  final DateTime createdAt;

  const TransactionEntity({
    required this.id,
    required this.type,
    required this.amount,
    required this.currency,
    required this.referenceId,
    required this.description,
    required this.createdAt,
  });

  /// True when this transaction credits the user's balance.
  bool get isCredit =>
      type == TransactionType.buy || type == TransactionType.earn;

  @override
  List<Object?> get props =>
      [id, type, amount, currency, referenceId, description, createdAt];
}
