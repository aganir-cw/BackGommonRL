import torch
import torch.nn as nn
import torch.nn.functional as F

class ValueNet(nn.Module):
    def __init__(self, hidden=80):
        super().__init__()
        self.network = nn.Sequential (
            nn.Linear(198, hidden),
            nn.ReLU(),
            nn.Linear(hidden, 1),
            nn.Sigmoid(),
        )

    def forward(self, x):
        return self.network(x)


